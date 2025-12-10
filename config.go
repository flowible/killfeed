package killfeed

import (
	"errors"
	"os"
)

type Config struct {
	Environment           string
	Port                  int
	EsiContactInformation string

	RedisURL string

	ZkillboardQueueID string
}

const (
	EnvironmentProduction = "production"
)

func NewConfig() (Config, error) {
	config := Config{
		Environment:           os.Getenv("ENVIRONMENT"),
		Port:                  8081,
		EsiContactInformation: os.Getenv("ESI_CONTACT_INFORMATION"),
		RedisURL:              os.Getenv("REDIS_URL"),
		ZkillboardQueueID:     os.Getenv("ZKILLBOARD_QUEUE_ID"),
	}

	if config.RedisURL == "" {
		return config, errors.New("missing redis url")
	}

	if config.EsiContactInformation == "" {
		return config, errors.New("missing ESI contact information")
	}

	if config.ZkillboardQueueID == "" {
		return config, errors.New("missing zkillboard queue ID")
	}

	return config, nil
}
