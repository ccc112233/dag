package mqtt

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	pahomqtt "github.com/eclipse/paho.mqtt.golang"
)

// Driver publishes envelopes to MQTT.
type Driver struct{}

func (Driver) Type() string { return "mqtt" }

type config struct {
	Broker   string `json:"broker"`
	ClientID string `json:"clientId"`
	Topic    string `json:"topic"` // supports {deviceId}
	QoS      byte   `json:"qos"`
	Username string `json:"username"`
	Password string `json:"password"`
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	if c.Broker == "" || c.Topic == "" {
		return fmt.Errorf("mqtt: broker and topic required")
	}
	return nil
}

func (Driver) Open(_ context.Context, cfg json.RawMessage, creds plugin.Credentials) (plugin.Publisher, error) {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.ClientID == "" {
		c.ClientID = fmt.Sprintf("dap-%d", time.Now().UnixNano())
	}
	if c.Username == "" {
		c.Username = creds["username"]
	}
	if c.Password == "" {
		c.Password = creds["password"]
		if c.Password == "" {
			c.Password = creds["token"]
		}
	}
	opts := pahomqtt.NewClientOptions().AddBroker(c.Broker).SetClientID(c.ClientID).SetAutoReconnect(true)
	if c.Username != "" {
		opts.SetUsername(c.Username)
		opts.SetPassword(c.Password)
	}
	client := pahomqtt.NewClient(opts)
	token := client.Connect()
	if !token.WaitTimeout(10 * time.Second) {
		return nil, fmt.Errorf("mqtt connect timeout")
	}
	if err := token.Error(); err != nil {
		return nil, fmt.Errorf("mqtt connect: %w", err)
	}
	return &publisher{client: client, cfg: c}, nil
}

type publisher struct {
	client pahomqtt.Client
	cfg    config
}

func (p *publisher) Publish(ctx context.Context, envelope plugin.Envelope) error {
	topic := strings.ReplaceAll(p.cfg.Topic, "{deviceId}", envelope.DeviceID)
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	token := p.client.Publish(topic, p.cfg.QoS, false, body)
	done := make(chan error, 1)
	go func() {
		token.Wait()
		done <- token.Error()
	}()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case err := <-done:
		return err
	}
}

func (p *publisher) Close() error {
	p.client.Disconnect(250)
	return nil
}
