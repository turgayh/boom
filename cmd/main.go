package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/turgayh/boom/internal/api"
	"github.com/turgayh/boom/internal/config"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
)

func main() {
	cfg, err := config.Load()
	must(err, "failed to load configuration")

	// Structured logging
	level := slog.LevelInfo
	if strings.EqualFold(cfg.LogLevel, "debug") {
		level = slog.LevelDebug
	}
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level})))

	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)
	ctx := context.Background()

	// Database
	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	must(err, "failed to connect to database")
	defer pool.Close()
	must(pool.Ping(ctx), "failed to ping database")
	must(runMigrations(ctx, pool), "failed to run migrations")
	slog.Info("database connected and migrated")

	// RabbitMQ
	conn, err := amqp.Dial(cfg.RabbitMQURL)
	must(err, "failed to connect to RabbitMQ")
	defer conn.Close()
	ch, err := conn.Channel()
	must(err, "failed to open channel")
	defer ch.Close()
	slog.Info("RabbitMQ connected")

	notificationRepository := repository.NewNotificationRepository(pool, logger)
	publisher := queue.NewPublisher(ch, logger)
	handler := api.NewHandler(notificationRepository, publisher)

	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	v1 := r.Group("v1")
	v1.POST("/notifications", handler.CreateNotification)
	v1.GET("/health", handler.Health)
	must(r.Run(":"+cfg.Port), "http server failed")
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	migration, err := os.ReadFile("migrations/001_init.up.sql")
	if err != nil {
		migration, err = os.ReadFile("/migrations/001_init.up.sql")
		if err != nil {
			return fmt.Errorf("read migration file: %w", err)
		}
	}

	_, err = pool.Exec(ctx, string(migration))
	if err != nil {
		if strings.Contains(err.Error(), "already exists") {
			return nil
		}
		return fmt.Errorf("execute migration: %w", err)
	}
	return nil
}

func must(err error, msg string) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}
