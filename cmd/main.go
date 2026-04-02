package main

import (
	"log/slog"
	"os"

	"github.com/turgayh/boom/internal/config"
)

func main() {
	cfg, err := config.Load()
	if err != nil {
		slog.Error("Failed to load configuration", "error", err)
		os.Exit(1)
	}
	slog.Info("Configuration loaded", "database_url", cfg.DatabaseURL, "redis_url", cfg.RedisURL, "webhook_url", cfg.WebhookURL, "port", cfg.Port, "log_level", cfg.LogLevel)
}
