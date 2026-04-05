package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type Publisher interface {
	Publish(ctx context.Context, topic string, message any) error
}

type amqpPublisher struct {
	ch  *amqp.Channel
	log *slog.Logger
}

func NewPublisher(ch *amqp.Channel, log *slog.Logger) Publisher {
	return &amqpPublisher{ch: ch, log: log}
}

// TODO: add routing key and other priority options
func (p *amqpPublisher) Publish(ctx context.Context, topic string, message any) error {
	body, err := json.Marshal(message)
	if err != nil {
		p.log.Error("failed to marshal message", "error", err)
		return err
	}
	err = p.ch.PublishWithContext(ctx, "", topic, false, false, amqp.Publishing{
		ContentType: "application/json",
		Body:        body,
	})
	if err != nil {
		p.log.Error("failed to publish message", "error", err)
		return err
	}
	return nil
}
