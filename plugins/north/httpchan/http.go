package httpchan

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
)

// Driver pushes JSON envelopes over HTTP(S).
type Driver struct{}

func (Driver) Type() string { return "http" }

type config struct {
	URL     string            `json:"url"`
	Method  string            `json:"method"`
	Headers map[string]string `json:"headers"`
	Timeout int               `json:"timeoutMs"`
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	if c.URL == "" {
		return fmt.Errorf("http: url required")
	}
	return nil
}

func (Driver) Open(_ context.Context, cfg json.RawMessage, creds plugin.Credentials) (plugin.Publisher, error) {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.Method == "" {
		c.Method = http.MethodPost
	}
	if c.Timeout <= 0 {
		c.Timeout = 5000
	}
	if c.Headers == nil {
		c.Headers = map[string]string{}
	}
	if tok := creds["token"]; tok != "" {
		if _, ok := c.Headers["Authorization"]; !ok {
			c.Headers["Authorization"] = "Bearer " + tok
		}
	}
	return &publisher{
		cfg:    c,
		client: &http.Client{Timeout: time.Duration(c.Timeout) * time.Millisecond},
	}, nil
}

type publisher struct {
	cfg    config
	client *http.Client
}

func (p *publisher) Publish(ctx context.Context, envelope plugin.Envelope) error {
	body, err := json.Marshal(envelope)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, p.cfg.Method, p.cfg.URL, bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	for k, v := range p.cfg.Headers {
		req.Header.Set(k, v)
	}
	resp, err := p.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode >= 300 {
		return fmt.Errorf("http publish status %d", resp.StatusCode)
	}
	return nil
}

func (p *publisher) Close() error { return nil }
