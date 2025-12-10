package main

import (
	"context"
	"encoding/json"
	"killfeed"
	"strings"
	"time"

	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

const GroupID = "killfeed:noopconsumer"
const ConsumerID = "any"

func main() {
	ctx := context.Background()

	log.Logger = log.Output(killfeed.LogOut{})
	zerolog.SetGlobalLevel(zerolog.DebugLevel)

	config, err := killfeed.NewConfig()
	if err != nil {
		log.Fatal().Err(err).Msg("failed to read config")
	}

	rdb := redis.NewClient(&redis.Options{Addr: config.RedisURL})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to connect to redis")
	}

	defer rdb.Close()

	if err := rdb.XGroupCreate(ctx, killfeed.StreamKillmails, GroupID, "$").Err(); err != nil && !strings.HasPrefix(err.Error(), "BUSYGROUP") {
		log.Fatal().Err(err).Msg("failed to create consumer group")
	}

	if err := rdb.XGroupCreateConsumer(ctx, killfeed.StreamKillmails, GroupID, ConsumerID).Err(); err != nil {
		log.Fatal().Err(err).Msg("failed to create consumer")
	}

	args := &redis.XReadGroupArgs{
		Group:    GroupID,
		Consumer: ConsumerID,
		Streams:  []string{killfeed.StreamKillmails, ">"},
		Count:    1,
		Block:    0,
		NoAck:    true,
	}

	for {
		responses, err := rdb.XReadGroup(ctx, args).Result()
		if err != nil {
			log.Error().Err(err).Msg("failed to read stream")
			time.Sleep(1 * time.Second)
			continue
		}

		for _, response := range responses {
			for _, message := range response.Messages {
				var killmail killfeed.Killmail
				if err := json.Unmarshal([]byte(message.Values["killmail"].(string)), &killmail); err != nil {
					log.Error().Str("message-id", message.ID).Err(err).Msg("failed to decode stream message")
					continue
				}

				log.Info().Str("message-id", message.ID).Int32("killmail-id", killmail.KillmailId).Msg("received stream message")
			}
		}
	}
}
