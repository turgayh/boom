package queue

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
	"github.com/turgayh/boom/internal/domain"
)

type Publisher interface {
	Publish(ctx context.Context, notification *domain.Notification) error
}

const (
	QueueHigh   = "notifications.high"
	QueueNormal = "notifications.normal"
	QueueLow    = "notifications.low"
)

type amqpPublisher struct {
	ch  *amqp.Channel
	log *slog.Logger
}

func NewPublisher(ch *amqp.Channel, log *slog.Logger) (Publisher, error) {
	for _, q := range []string{QueueHigh, QueueNormal, QueueLow} {
		_, err := ch.QueueDeclare(q, true, false, false, false, amqp.Table{
			"x-dead-letter-exchange":    "",
			"x-dead-letter-routing-key": q + ".dlq",
		})
		if err != nil {
			return nil, fmt.Errorf("declare queue %s: %w", q, err)
		}
		_, err = ch.QueueDeclare(q+".dlq", true, false, false, false, nil)
		if err != nil {
			return nil, fmt.Errorf("declare DLQ %s: %w", q+".dlq", err)
		}
	}
	return &amqpPublisher{ch: ch, log: log}, nil
}

func (p *amqpPublisher) Publish(ctx context.Context, notification *domain.Notification) error {
	q := priorityToQueue(notification.Priority)

	body, err := json.Marshal(notification)
	if err != nil {
		return fmt.Errorf("marshal notification: %w", err)
	}

	p.log.Info("publishing notification", "id", notification.ID, "queue", q, "priority", notification.Priority)

	return p.ch.PublishWithContext(ctx, "", q, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
}

func priorityToQueue(p domain.Priority) string {
	switch p {
	case domain.PriorityHigh:
		return QueueHigh
	case domain.PriorityLow:
		return QueueLow
	default:
		return QueueNormal
	}
}
