package queue

import (
	"context"
	"encoding/json"
	"log/slog"

	amqp "github.com/rabbitmq/amqp091-go"
)

type IPublisher interface {
	Publish(ctx context.Context, topic string, message any) error
}

type Publisher struct {
	ch  *amqp.Channel
	log *slog.Logger
}

func NewPublisher(ch *amqp.Channel, log *slog.Logger) IPublisher {
	return &Publisher{ch: ch}
}

// TODO: add routing key and other priority options
func (p *Publisher) Publish(ctx context.Context, topic string, message any) error {
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
