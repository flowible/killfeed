package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"killfeed"
	"killfeed/httperror"
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/go-chi/render"
	"github.com/olahol/melody"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

func main() {
	ctx := context.Background()

	log.Logger = log.Output(killfeed.LogOut{})

	config, err := killfeed.NewConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config")
	}

	rdb := redis.NewClient(&redis.Options{Addr: config.RedisURL})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}

	defer rdb.Close()

	m := melody.New()

	// No limit on messages
	m.Config.MaxMessageSize = 0
	m.Config.WriteWait = 5 * time.Second

	r := NewRouter()
	r.Use(middleware.Logger)

	r.Get("/_healthz", func(w http.ResponseWriter, r *http.Request) *httperror.HTTPError {
		w.WriteHeader(http.StatusOK)
		return nil
	})

	r.Get("/version", func(w http.ResponseWriter, r *http.Request) *httperror.HTTPError {
		w.Write([]byte(killfeed.Version))
		w.WriteHeader(http.StatusOK)
		return nil
	})

	r.Get("/", func(w http.ResponseWriter, r *http.Request) *httperror.HTTPError {
		render.HTML(w, r, "<html><body>Hello</body></html>")
		return nil
	})

	r.Get("/websocket/{queueID}", func(w http.ResponseWriter, r *http.Request) *httperror.HTTPError {
		queueID := chi.URLParam(r, "queueID")
		if len(queueID) > 128 {
			return httperror.BadRequest("queue ID must be 128 characters or less")
		}

		m.HandleRequestWithKeys(w, r, map[string]any{"queueID": queueID})
		return nil
	})

	r.Get("/poll/{queueID}", func(w http.ResponseWriter, r *http.Request) *httperror.HTTPError {
		ctx := r.Context()

		queueID := chi.URLParam(r, "queueID")
		if len(queueID) > 128 {
			return httperror.BadRequest("queue ID must be 128 characters or less")
		}

		latestIDKey := fmt.Sprintf("stream:poll:%s", queueID)

		latestID, err := rdb.Get(ctx, latestIDKey).Result()
		if err != nil && err != redis.Nil {
			return httperror.InternalServerError("failed to get latest ID from redis", err)
		}

		if latestID == "" {
			latestID = "$"
		}

		killmails := []killfeed.CombinedKillmail{}

		args := &redis.XReadArgs{
			ID:      latestID,
			Streams: []string{killfeed.StreamKillmails},
			Count:   100,
			Block:   60 * time.Second,
		}

		streams, err := rdb.XRead(ctx, args).Result()
		if err == redis.Nil {
			render.JSON(w, r, killmails)
			return nil
		}

		if err != nil {
			return httperror.InternalServerError("failed to read from redis stream", err)
		}

		for _, stream := range streams {
			for _, message := range stream.Messages {
				latestID = message.ID

				encodedKillmail, ok := message.Values["killmail"].(string)
				if !ok {
					return httperror.InternalServerError(fmt.Sprintf("invalid killmail message %s", message.ID), err)
				}

				encodedKillmailZkb, ok := message.Values["killmail_zkb"].(string)
				if !ok {
					return httperror.InternalServerError(fmt.Sprintf("invalid killmail message %s, missing zkb fields", message.ID), err)
				}

				var killmail killfeed.Killmail
				if err := json.Unmarshal([]byte(encodedKillmail), &killmail); err != nil {
					return httperror.InternalServerError(fmt.Sprintf("failed to decode killmail message %s", message.ID), err)
				}

				var killmailZkb killfeed.KillmailZkb
				if err := json.Unmarshal([]byte(encodedKillmailZkb), &killmailZkb); err != nil {
					return httperror.InternalServerError(fmt.Sprintf("failed to decode killmail message %s zkb fields", message.ID), err)
				}

				killmails = append(killmails, killfeed.CombinedKillmail{
					Attackers:     killmail.Attackers,
					KillmailId:    killmail.KillmailId,
					KillmailTime:  killmail.KillmailTime,
					MoonId:        killmail.MoonId,
					SolarSystemId: killmail.SolarSystemId,
					Victim:        killmail.Victim,
					WarId:         killmail.WarId,
					Zkb:           killmailZkb,
				})
			}
		}

		if err := rdb.Set(ctx, latestIDKey, latestID, 24*time.Hour).Err(); err != nil {
			return httperror.InternalServerError("failed to store latest ID to redis", err)
		}

		render.JSON(w, r, killmails)
		return nil
	})

	m.HandleConnect(func(s *melody.Session) {
		queueID := s.Keys["queueID"].(string)

		log.Info().Str("queueID", queueID).Msg("new websocket connection")

		go handleWebsocket(s.Request.Context(), log.With().Str("queue-id", queueID).Logger(), rdb, s, queueID)
	})

	m.HandleDisconnect(func(s *melody.Session) {
		queueID := s.Keys["queueID"].(string)

		log.Info().Str("queueID", queueID).Msg("closed websocket connection")
	})

	log.Info().Int("port", config.Port).Msg("http server listening")

	srv := &http.Server{Addr: fmt.Sprintf(":%d", config.Port), Handler: r}
	if err := srv.ListenAndServe(); err != nil {
		log.Fatal().Err(err).Msg("http listener failed")
	}
}

func handleWebsocket(ctx context.Context, logger zerolog.Logger, rdb *redis.Client, s *melody.Session, queueID string) {
	for {
		select {
		case <-ctx.Done():
			return

		default:
			messages, err := fetchWebsocketKillmails(ctx, rdb, queueID)
			if err != nil {
				if errors.Is(err, context.Canceled) && s.IsClosed() {
					return
				}

				logger.Error().Err(err).Msg("failed to fetch websocket killmails")
				if err := s.CloseWithMsg(melody.FormatCloseMessage(melody.CloseInternalServerErr, "internal server error")); err != nil {
					logger.Error().Err(err).Msg("failed to close websocket after fetch error")
				}

				return
			}

			for _, message := range messages {
				if err := s.WriteWithDeadline(message, 0); err != nil {
					logger.Error().Err(err).Msg("failed to write to websocket")
					if err := s.CloseWithMsg(melody.FormatCloseMessage(melody.CloseAbnormalClosure, "write failed")); err != nil {
						logger.Error().Err(err).Msg("failed to close websocket after write error")
					}
				}
			}
		}
	}
}

func fetchWebsocketKillmails(ctx context.Context, rdb *redis.Client, queueID string) ([][]byte, error) {
	latestIDKey := fmt.Sprintf("stream:websocket:%s", queueID)

	latestID, err := rdb.Get(ctx, latestIDKey).Result()
	if err != nil && err != redis.Nil {
		return nil, fmt.Errorf("failed to get latest ID from redis: %w", err)
	}

	if latestID == "" {
		latestID = "$"
	}

	killmails := [][]byte{}

	args := &redis.XReadArgs{
		ID:      latestID,
		Streams: []string{killfeed.StreamKillmails},
		Count:   10,
		Block:   0,
	}

	streams, err := rdb.XRead(ctx, args).Result()
	if err == redis.Nil {
		return killmails, nil
	}

	if err != nil {
		return nil, fmt.Errorf("failed to read from redis stream: %w", err)
	}

	for _, stream := range streams {
		for _, message := range stream.Messages {
			latestID = message.ID

			encodedKillmail, ok := message.Values["killmail"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid killmail message %s", message.ID)
			}

			encodedKillmailZkb, ok := message.Values["killmail_zkb"].(string)
			if !ok {
				return nil, fmt.Errorf("invalid killmail message %s, missing zkb fields", message.ID)
			}

			var killmail killfeed.Killmail
			if err := json.Unmarshal([]byte(encodedKillmail), &killmail); err != nil {
				return nil, fmt.Errorf("failed to decode killmail message %s: %w", message.ID, err)
			}

			var killmailZkb killfeed.KillmailZkb
			if err := json.Unmarshal([]byte(encodedKillmailZkb), &killmailZkb); err != nil {
				return nil, fmt.Errorf("failed to decode killmail message %s zkb fields: %w", message.ID, err)
			}

			combinedKillmail := killfeed.CombinedKillmail{
				Attackers:     killmail.Attackers,
				KillmailId:    killmail.KillmailId,
				KillmailTime:  killmail.KillmailTime,
				MoonId:        killmail.MoonId,
				SolarSystemId: killmail.SolarSystemId,
				Victim:        killmail.Victim,
				WarId:         killmail.WarId,
				Zkb:           killmailZkb,
			}

			payload, err := json.Marshal(combinedKillmail)
			if err != nil {
				return nil, fmt.Errorf("failed to encode combined killmail %s: %w", message.ID, err)
			}

			killmails = append(killmails, payload)
		}
	}

	if err := rdb.Set(ctx, latestIDKey, latestID, 24*time.Hour).Err(); err != nil {
		return nil, fmt.Errorf("failed to store latest ID to redis: %w", err)
	}

	return killmails, nil
}
