package plugin

import (
	"context"
	"encoding/json"
	"time"
)

// Quality for samples / mapped properties.
type Quality string

const (
	QualityGood      Quality = "good"
	QualityBad       Quality = "bad"
	QualityUncertain Quality = "uncertain"
)

// PointAddr is a protocol-agnostic point address.
type PointAddr struct {
	ID      string          `json:"id"`
	Name    string          `json:"name,omitempty"`
	Address json.RawMessage `json:"address"`
	DataType string         `json:"dataType,omitempty"`
}

// Sample is a single south-bound reading.
type Sample struct {
	PointID   string    `json:"pointId"`
	Address   string    `json:"address,omitempty"`
	Value     any       `json:"value"`
	Timestamp time.Time `json:"timestamp"`
	Quality   Quality   `json:"quality"`
}

// PropertyValue is a mapped property in the north envelope.
type PropertyValue struct {
	Value   any     `json:"value"`
	Quality Quality `json:"quality"`
}

// Envelope is the unified northbound telemetry payload.
type Envelope struct {
	MessageID    string                    `json:"messageId"`
	DeviceID     string                    `json:"deviceId"`
	ModelID      string                    `json:"modelId"`
	ModelVersion string                    `json:"modelVersion"`
	Timestamp    time.Time                 `json:"timestamp"`
	Properties   map[string]PropertyValue  `json:"properties"`
	Source       string                    `json:"source,omitempty"`
}

// Credentials resolves a credentials_ref (env name or file path).
type Credentials map[string]string

// Session is an open south connection.
type Session interface {
	Read(ctx context.Context, points []PointAddr) ([]Sample, error)
	Subscribe(ctx context.Context, points []PointAddr, sink chan<- Sample) (Subscription, error)
	Close() error
}

// Subscription cancels a subscribe loop.
type Subscription interface {
	Unsubscribe() error
}

// SouthDriver is the southbound plugin contract.
type SouthDriver interface {
	Type() string
	ValidateConfig(cfg json.RawMessage) error
	Open(ctx context.Context, cfg json.RawMessage, creds Credentials) (Session, error)
}

// Publisher is an open north channel.
type Publisher interface {
	Publish(ctx context.Context, envelope Envelope) error
	Close() error
}

// NorthChannel is the northbound plugin contract.
type NorthChannel interface {
	Type() string
	ValidateConfig(cfg json.RawMessage) error
	Open(ctx context.Context, cfg json.RawMessage, creds Credentials) (Publisher, error)
}
