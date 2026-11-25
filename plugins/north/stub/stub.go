package stub

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"

	"github.com/ccc112233/dag/internal/plugin"
)

// Driver writes envelopes to memory and optional file for tests.
type Driver struct{}

func (Driver) Type() string { return "stub_north" }

type config struct {
	Path      string `json:"path"`
	FailOnce  bool   `json:"failOnce"`
	FailTimes int    `json:"failTimes"`
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if len(cfg) == 0 {
		return nil
	}
	return json.Unmarshal(cfg, &c)
}

var (
	globalMu      sync.Mutex
	globalRing    []plugin.Envelope
	failRemaining atomic.Int32
	failSeeded    atomic.Bool
)

// Reset clears in-memory ring (tests).
func Reset() {
	globalMu.Lock()
	defer globalMu.Unlock()
	globalRing = nil
	failRemaining.Store(0)
	failSeeded.Store(false)
}

// Allow clears remaining forced publish failures (tests).
func Allow() {
	failRemaining.Store(0)
}

// Snapshots returns a copy of received envelopes.
func Snapshots() []plugin.Envelope {
	globalMu.Lock()
	defer globalMu.Unlock()
	out := make([]plugin.Envelope, len(globalRing))
	copy(out, globalRing)
	return out
}

func (Driver) Open(_ context.Context, cfg json.RawMessage, _ plugin.Credentials) (plugin.Publisher, error) {
	c := config{}
	_ = json.Unmarshal(cfg, &c)
	// Seed forced failures once per Reset(); re-Open after InvalidatePublisher must not reset Allow().
	if failSeeded.CompareAndSwap(false, true) {
		if c.FailTimes > 0 {
			failRemaining.Store(int32(c.FailTimes))
		} else if c.FailOnce {
			failRemaining.Store(1)
		}
	}
	return &publisher{cfg: c}, nil
}

type publisher struct {
	cfg config
	mu  sync.Mutex
}

func (p *publisher) Publish(_ context.Context, envelope plugin.Envelope) error {
	p.mu.Lock()
	defer p.mu.Unlock()
	for {
		n := failRemaining.Load()
		if n <= 0 {
			break
		}
		if failRemaining.CompareAndSwap(n, n-1) {
			return errFailOnce
		}
	}
	globalMu.Lock()
	globalRing = append(globalRing, envelope)
	if len(globalRing) > 1000 {
		globalRing = globalRing[len(globalRing)-1000:]
	}
	globalMu.Unlock()
	if p.cfg.Path != "" {
		_ = os.MkdirAll(filepath.Dir(p.cfg.Path), 0o755)
		b, _ := json.Marshal(envelope)
		f, err := os.OpenFile(p.cfg.Path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
		if err == nil {
			_, _ = f.Write(append(b, '\n'))
			_ = f.Close()
		}
	}
	return nil
}

func (p *publisher) Close() error { return nil }

type failOnceError struct{}

func (failOnceError) Error() string { return "stub_north: failOnce" }

var errFailOnce = failOnceError{}
