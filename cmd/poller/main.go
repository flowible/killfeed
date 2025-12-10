package main

import (
	"context"
	"encoding/json"
	"fmt"
	"killfeed"
	"net/http"
	"time"

	"github.com/antihax/goesi"
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

	httpClient := &http.Client{Timeout: 10 * time.Second}

	esiClient := goesi.NewAPIClient(httpClient, fmt.Sprintf("Killfeed/%s (%s)", killfeed.Version, config.EsiContactInformation))

	go watchRedisQ(ctx, log.With().Str("source", "redisq").Logger(), rdb, esiClient, config.ZkillboardQueueID)

	<-make(chan bool, 1)
}

func processKillmail(ctx context.Context, logger zerolog.Logger, rdb *redis.Client, esiClient *goesi.APIClient, killmailID int32, killmailZkb killfeed.KillmailZkb) {
	killmail, _, err := esiClient.ESI.KillmailsApi.GetKillmailsKillmailIdKillmailHash(ctx, killmailZkb.Hash, killmailID, nil)
	if err != nil {
		logger.Error().Err(err).Msg("failed to fetch killmail from ESI")
		return
	}

	encodedKillmail, err := json.Marshal(killmail)
	if err != nil {
		logger.Error().Err(err).Msg("failed to encode killmail")
		return
	}

	encodedKillmailZkb, err := json.Marshal(killmailZkb)
	if err != nil {
		logger.Error().Err(err).Msg("failed to encode killmail zkb")
		return
	}

	args := &redis.XAddArgs{
		Stream: killfeed.StreamKillmails,
		ID:     "*",
		MaxLen: killfeed.StreamMaxLength,
		Approx: true,
		Values: map[string]any{
			"killmail":     string(encodedKillmail),
			"killmail_zkb": string(encodedKillmailZkb),
		},
	}

	if err := rdb.XAdd(ctx, args).Err(); err != nil {
		logger.Error().Err(err).Msg("failed to add killmail to queue")
	}
}
