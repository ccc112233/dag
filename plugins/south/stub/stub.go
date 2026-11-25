package stub

import (
	"context"
	"encoding/json"
	"fmt"
	"sync/atomic"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
)

// Driver is a south stub that emits incrementing values.
type Driver struct{}

func (Driver) Type() string { return "stub_south" }

type config struct {
	StartValue float64 `json:"startValue"`
	Step       float64 `json:"step"`
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if len(cfg) == 0 {
		return nil
	}
	return json.Unmarshal(cfg, &c)
}

func (Driver) Open(_ context.Context, cfg json.RawMessage, _ plugin.Credentials) (plugin.Session, error) {
	c := config{StartValue: 0, Step: 1}
	if len(cfg) > 0 {
		_ = json.Unmarshal(cfg, &c)
	}
	s := &session{step: c.Step}
	s.cur.Store(c.StartValue)
	return s, nil
}

type session struct {
	cur  atomic.Value // float64
	step float64
}

func (s *session) Read(_ context.Context, points []plugin.PointAddr) ([]plugin.Sample, error) {
	now := time.Now().UTC()
	out := make([]plugin.Sample, 0, len(points))
	for _, p := range points {
		v := s.cur.Load().(float64)
		next := v + s.step
		s.cur.Store(next)
		out = append(out, plugin.Sample{
			PointID:   p.ID,
			Address:   string(p.Address),
			Value:     v,
			Timestamp: now,
			Quality:   plugin.QualityGood,
		})
	}
	return out, nil
}

func (s *session) Subscribe(ctx context.Context, points []plugin.PointAddr, sink chan<- plugin.Sample) (plugin.Subscription, error) {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(200 * time.Millisecond)
		defer t.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-stop:
				return
			case <-t.C:
				samples, err := s.Read(ctx, points)
				if err != nil {
					continue
				}
				for _, sm := range samples {
					select {
					case sink <- sm:
					case <-ctx.Done():
						return
					case <-stop:
						return
					}
				}
			}
		}
	}()
	return sub{stop: stop}, nil
}

func (s *session) Close() error { return nil }

type sub struct{ stop chan struct{} }

func (s sub) Unsubscribe() error {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	return nil
}

// FailDriver always fails Open/Read — used for isolation tests.
type FailDriver struct{}

func (FailDriver) Type() string { return "stub_south_fail" }

func (FailDriver) ValidateConfig(json.RawMessage) error { return nil }

func (FailDriver) Open(context.Context, json.RawMessage, plugin.Credentials) (plugin.Session, error) {
	return failSession{}, nil
}

type failSession struct{}

func (failSession) Read(context.Context, []plugin.PointAddr) ([]plugin.Sample, error) {
	return nil, fmt.Errorf("stub_south_fail: intentional read failure")
}
func (failSession) Subscribe(context.Context, []plugin.PointAddr, chan<- plugin.Sample) (plugin.Subscription, error) {
	return nil, fmt.Errorf("stub_south_fail: intentional subscribe failure")
}
func (failSession) Close() error { return nil }
