package amqp

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	amqp "github.com/rabbitmq/amqp091-go"
)

// Driver publishes to RabbitMQ (AMQP 0-9-1).
type Driver struct{}

func (Driver) Type() string { return "amqp" }

type config struct {
	URL        string `json:"url"` // amqp://guest:guest@localhost:5672/
	Exchange   string `json:"exchange"`
	RoutingKey string `json:"routingKey"`
	Queue      string `json:"queue"` // if set, declare and bind
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	if c.URL == "" {
		return fmt.Errorf("amqp: url required")
	}
	return nil
}

func (Driver) Open(ctx context.Context, cfg json.RawMessage, _ plugin.Credentials) (plugin.Publisher, error) {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	conn, err := amqp.Dial(c.URL)
	if err != nil {
		return nil, fmt.Errorf("amqp dial: %w", err)
	}
	ch, err := conn.Channel()
	if err != nil {
		_ = conn.Close()
		return nil, err
	}
	if c.Queue != "" {
		_, err = ch.QueueDeclare(c.Queue, true, false, false, false, nil)
		if err != nil {
			_ = ch.Close()
			_ = conn.Close()
			return nil, err
		}
		if c.Exchange != "" {
			if err := ch.ExchangeDeclare(c.Exchange, "topic", true, false, false, false, nil); err != nil {
				_ = ch.Close()
				_ = conn.Close()
				return nil, err
			}
			rk := c.RoutingKey
			if rk == "" {
				rk = c.Queue
			}
			if err := ch.QueueBind(c.Queue, rk, c.Exchange, false, nil); err != nil {
				_ = ch.Close()
				_ = conn.Close()
				return nil, err
			}
		}
	}
	_ = ctx
	return &publisher{conn: conn, ch: ch, cfg: c}, nil
}

type publisher struct {
	conn *amqp.Connection
	ch   *amqp.Channel
	cfg  config
}

func (p *publisher) Publish(ctx context.Context, envelope plugin.Envelope) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	rk := p.cfg.RoutingKey
	if rk == "" {
		rk = p.cfg.Queue
	}
	ex := p.cfg.Exchange
	pubCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	return p.ch.PublishWithContext(pubCtx, ex, rk, false, false, amqp.Publishing{
		ContentType:  "application/json",
		Body:         body,
		DeliveryMode: amqp.Persistent,
		Timestamp:    time.Now().UTC(),
		MessageId:    envelope.MessageID,
	})
}

func (p *publisher) Close() error {
	_ = p.ch.Close()
	return p.conn.Close()
}
