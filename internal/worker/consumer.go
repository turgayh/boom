package worker

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/turgayh/boom/internal/domain"
	"github.com/turgayh/boom/internal/provider"
	"github.com/turgayh/boom/internal/queue"
	"github.com/turgayh/boom/internal/repository"
	"golang.org/x/time/rate"
)

const maxRetries = 3

type Worker struct {
	ch       *amqp.Channel
	repo     repository.NotificationRepository
	sender   provider.Sender
	log      *slog.Logger
	limiters map[string]*rate.Limiter //IMPROVEMENT: distributed rate limiting using Redis
}

func New(ch *amqp.Channel, repo repository.NotificationRepository, sender provider.Sender, log *slog.Logger) *Worker {
	return &Worker{
		ch:     ch,
		repo:   repo,
		sender: sender,
		log:    log,
		limiters: map[string]*rate.Limiter{
			"sms":   rate.NewLimiter(rate.Limit(100), 100), //IMPROVEMENT: configurable limits for each channel
			"email": rate.NewLimiter(rate.Limit(100), 100), //IMPROVEMENT: configurable limits for each channel
			"push":  rate.NewLimiter(rate.Limit(100), 100), //IMPROVEMENT: configurable limits for each channel
		},
	}
}

// IMPROVEMENT: We can add weighted consumers for each queue to balance the load (e.g. high priority queue gets more consumers).
func (w *Worker) Start(ctx context.Context) error {
	queues := []string{queue.QueueHigh, queue.QueueNormal, queue.QueueLow}

	var wg sync.WaitGroup
	for _, q := range queues {
		msgs, err := w.ch.Consume(q, "", false, false, false, false, nil)
		if err != nil {
			return fmt.Errorf("consume %s: %w", q, err)
		}

		wg.Add(1)
		go func(queueName string, deliveries <-chan amqp.Delivery) {
			defer wg.Done()
			w.log.Info("worker started", "queue", queueName)

			for {
				select {
				case <-ctx.Done():
					w.log.Info("worker stopping", "queue", queueName)
					return
				case msg, ok := <-deliveries:
					if !ok {
						w.log.Info("channel closed", "queue", queueName)
						return
					}
					w.process(ctx, msg)
				}
			}
		}(q, msgs)
	}

	wg.Wait()
	return nil
}

func (w *Worker) process(ctx context.Context, msg amqp.Delivery) {
	var notification domain.Notification
	if err := json.Unmarshal(msg.Body, &notification); err != nil {
		w.log.Error("unmarshal failed, discarding message", "error", err)
		msg.Nack(false, false)
		return
	}

	log := w.log.With("id", notification.ID, "channel", notification.Channel)

	// update status to processing
	if err := w.repo.UpdateStatus(ctx, notification.ID, domain.StatusProcessing, nil); err != nil {
		log.Error("failed to update status to processing", "error", err)
		msg.Nack(false, true)
		return
	}

	// rate limit per channel
	limiter, ok := w.limiters[notification.Channel]
	if ok {
		if err := limiter.Wait(ctx); err != nil {
			log.Error("rate limiter error", "error", err)
			msg.Nack(false, true)
			return
		}
	}

	// retry with exponential backoff (max 3 attempts)
	var resp *provider.SendResponse
	var lastErr error
	for attempt := 1; attempt <= maxRetries; attempt++ {
		resp, lastErr = w.sender.Send(ctx, &provider.SendRequest{
			To:      notification.Recipient,
			Channel: notification.Channel,
			Content: notification.Content,
		})
		if lastErr == nil {
			break
		}

		log.Warn("delivery attempt failed", "attempt", attempt, "max", maxRetries, "error", lastErr)

		if attempt < maxRetries {
			backoff := time.Duration(1<<uint(attempt-1)) * time.Second // 1s, 2s, 4s
			select {
			case <-ctx.Done():
				msg.Nack(false, true)
				return
			case <-time.After(backoff):
			}
		}
	}

	if lastErr != nil {
		log.Error("delivery failed after retries", "attempts", maxRetries, "error", lastErr)
		w.repo.UpdateStatus(ctx, notification.ID, domain.StatusFailed, nil)
		msg.Nack(false, false) // goes to DLQ
		return
	}

	// success
	w.repo.UpdateStatus(ctx, notification.ID, domain.StatusDelivered, &resp.MessageID)
	msg.Ack(false)
	log.Info("notification delivered", "provider_msg_id", resp.MessageID)
}
