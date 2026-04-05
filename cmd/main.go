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
	"github.com/turgayh/boom/internal/provider"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
	"github.com/turgayh/boom/internal/worker"
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
	publisher, err := queue.NewPublisher(ch, logger)
	must(err, "failed to initialize publisher")
	handler := api.NewHandler(notificationRepository, publisher)

	workerCh, err := conn.Channel()
	must(err, "failed to open worker channel")
	defer workerCh.Close()

	sender := provider.NewSender(cfg.WebhookURL)
	w := worker.New(workerCh, notificationRepository, sender, logger)

	workerCtx, workerCancel := context.WithCancel(ctx)
	defer workerCancel()

	go func() {
		if err := w.Start(workerCtx); err != nil {
			slog.Error("worker error", "error", err)
		}
	}()
	slog.Info("worker started")

	r := gin.Default()
	r.Use(gin.Recovery())
	r.Use(gin.Logger())
	v1 := r.Group("v1")
	v1.POST("/notifications", handler.CreateNotification)
	v1.GET("/health", handler.Health)
	must(r.Run(":"+cfg.Port), "http server failed")
}

func runMigrations(ctx context.Context, pool *pgxpool.Pool) error {
	files := []string{
		"migrations/001_init.up.sql",
		"migrations/002_add_provider_msg_id.up.sql",
	}

	for _, f := range files {
		migration, err := os.ReadFile(f)
		if err != nil {
			// try absolute path for Docker
			migration, err = os.ReadFile("/" + f)
			if err != nil {
				return fmt.Errorf("read migration file %s: %w", f, err)
			}
		}

		_, err = pool.Exec(ctx, string(migration))
		if err != nil {
			if strings.Contains(err.Error(), "already exists") {
				continue
			}
			return fmt.Errorf("execute migration %s: %w", f, err)
		}
	}
	return nil
}

func must(err error, msg string) {
	if err != nil {
		slog.Error(msg, "error", err)
		os.Exit(1)
	}
}
