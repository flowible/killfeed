package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"killfeed"
	"net/http"
	"time"

	"github.com/antihax/goesi"
	"github.com/redis/go-redis/v9"
	"github.com/rs/zerolog"
)

type RedisQPackage struct {
	KillID int32                `json:"killID"`
	Zkb    killfeed.KillmailZkb `json:"zkb"`
}

type RedisQResponse struct {
	Package *RedisQPackage `json:"package"`
}

func watchRedisQ(ctx context.Context, logger zerolog.Logger, rdb *redis.Client, esiClient *goesi.APIClient, queueID string) {
	for {
		response, err := fetchRedisQ(ctx, logger, queueID)
		if err != nil {
			logger.Error().Err(err).Msg("failed to fetch")

			// Sleep with context cancellation
			select {
			case <-ctx.Done():
			case <-time.After(10 * time.Second):
			}

			continue
		}

		if response.Package == nil {
			continue
		}

		if isKillmailCached(response.Package.KillID) {
			continue
		}

		go processKillmail(ctx, logger.With().Int32("killmail-id", response.Package.KillID).Logger(), rdb, esiClient, response.Package.KillID, response.Package.Zkb)
	}
}

func fetchRedisQ(ctx context.Context, logger zerolog.Logger, queueID string) (RedisQResponse, error) {
	httpClient := &http.Client{
		Timeout: 15 * time.Second,
	}

	req, err := http.NewRequestWithContext(ctx, "GET", fmt.Sprintf("https://zkillredisq.stream/listen.php?queueID=%s&ttw=10", queueID), nil)
	if err != nil {
		logger.Fatal().Err(err).Msg("failed to create request")
	}

	res, err := httpClient.Do(req)
	if err != nil {
		return RedisQResponse{}, fmt.Errorf("failed to send request: %w", err)
	}

	defer res.Body.Close()

	body, err := io.ReadAll(res.Body)
	if err != nil {
		return RedisQResponse{}, fmt.Errorf("failed to read response: %w", err)
	}

	if res.StatusCode != http.StatusOK {
		return RedisQResponse{}, fmt.Errorf("unexpected status code: %d", res.StatusCode)
	}

	var response RedisQResponse
	if err := json.Unmarshal(body, &response); err != nil {
		return RedisQResponse{}, fmt.Errorf("failed to decode response: %w", err)
	}

	return response, nil
}
