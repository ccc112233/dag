package opcua

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	"github.com/gopcua/opcua"
	"github.com/gopcua/opcua/ua"
)

// Driver implements OPC UA Client southbound.
type Driver struct{}

func (Driver) Type() string { return "opcua" }

type config struct {
	Endpoint       string `json:"endpoint"`
	SecurityPolicy string `json:"securityPolicy"`
	SecurityMode   string `json:"securityMode"`
	Username       string `json:"username"`
	Password       string `json:"password"`
	Insecure       bool   `json:"insecure"`
}

type pointAddr struct {
	NodeID string `json:"nodeId"`
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	if c.Endpoint == "" {
		return fmt.Errorf("opcua: endpoint required")
	}
	return nil
}

func (Driver) Open(ctx context.Context, cfg json.RawMessage, creds plugin.Credentials) (plugin.Session, error) {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
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
	opts := []opcua.Option{}
	if c.Insecure || c.SecurityMode == "" || c.SecurityMode == "None" {
		opts = append(opts,
			opcua.SecurityMode(ua.MessageSecurityModeNone),
			opcua.SecurityPolicy(ua.SecurityPolicyURINone),
		)
	}
	if c.Username != "" {
		opts = append(opts, opcua.AuthUsername(c.Username, c.Password))
	}
	client, err := opcua.NewClient(c.Endpoint, opts...)
	if err != nil {
		return nil, err
	}
	if err := client.Connect(ctx); err != nil {
		return nil, fmt.Errorf("opcua connect: %w", err)
	}
	return &session{client: client}, nil
}

type session struct {
	client *opcua.Client
}

func (s *session) Read(ctx context.Context, points []plugin.PointAddr) ([]plugin.Sample, error) {
	now := time.Now().UTC()
	out := make([]plugin.Sample, 0, len(points))
	for _, p := range points {
		var pa pointAddr
		if err := json.Unmarshal(p.Address, &pa); err != nil || pa.NodeID == "" {
			out = append(out, plugin.Sample{PointID: p.ID, Timestamp: now, Quality: plugin.QualityBad})
			continue
		}
		id, err := ua.ParseNodeID(pa.NodeID)
		if err != nil {
			out = append(out, plugin.Sample{PointID: p.ID, Address: pa.NodeID, Timestamp: now, Quality: plugin.QualityBad})
			continue
		}
		req := &ua.ReadRequest{
			MaxAge: 2000,
			NodesToRead: []*ua.ReadValueID{
				{NodeID: id},
			},
			TimestampsToReturn: ua.TimestampsToReturnBoth,
		}
		resp, err := s.client.Read(ctx, req)
		sm := plugin.Sample{PointID: p.ID, Address: pa.NodeID, Timestamp: now, Quality: plugin.QualityGood}
		if err != nil || resp == nil || len(resp.Results) == 0 {
			sm.Quality = plugin.QualityBad
			out = append(out, sm)
			continue
		}
		dv := resp.Results[0]
		if dv.Status != ua.StatusOK {
			sm.Quality = plugin.QualityBad
			out = append(out, sm)
			continue
		}
		if !dv.SourceTimestamp.IsZero() {
			sm.Timestamp = dv.SourceTimestamp.UTC()
		} else if !dv.ServerTimestamp.IsZero() {
			sm.Timestamp = dv.ServerTimestamp.UTC()
		}
		if dv.Value != nil {
			sm.Value = dv.Value.Value()
		}
		out = append(out, sm)
	}
	return out, nil
}

func (s *session) Subscribe(ctx context.Context, points []plugin.PointAddr, sink chan<- plugin.Sample) (plugin.Subscription, error) {
	// MVP: poll-based subscribe for reliability across security modes.
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(500 * time.Millisecond)
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

func (s *session) Close() error {
	return s.client.Close(context.Background())
}

type sub struct{ stop chan struct{} }

func (s sub) Unsubscribe() error {
	select {
	case <-s.stop:
	default:
		close(s.stop)
	}
	return nil
}
