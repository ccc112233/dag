package modbus

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"math"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	"github.com/goburrow/modbus"
)

// Driver implements Modbus TCP southbound.
type Driver struct{}

func (Driver) Type() string { return "modbus_tcp" }

type config struct {
	Address string `json:"address"` // host:port
	SlaveID byte   `json:"slaveId"`
	Timeout int    `json:"timeoutMs"`
}

type pointAddr struct {
	RegisterType string `json:"registerType"` // holding|input|coil|discrete
	Address      uint16 `json:"address"`
	Quantity     uint16 `json:"quantity"`
	DataType     string `json:"dataType"` // uint16|int16|uint32|float32|bool
	ByteOrder    string `json:"byteOrder"` // big|abcd|cdab|badc|dcba
}

func (Driver) ValidateConfig(cfg json.RawMessage) error {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return err
	}
	if c.Address == "" {
		return fmt.Errorf("modbus_tcp: address required")
	}
	return nil
}

func (Driver) Open(ctx context.Context, cfg json.RawMessage, _ plugin.Credentials) (plugin.Session, error) {
	var c config
	if err := json.Unmarshal(cfg, &c); err != nil {
		return nil, err
	}
	if c.SlaveID == 0 {
		c.SlaveID = 1
	}
	if c.Timeout <= 0 {
		c.Timeout = 3000
	}
	if err := ctx.Err(); err != nil {
		return nil, fmt.Errorf("modbus_tcp connect: %w", err)
	}
	handler := modbus.NewTCPClientHandler(c.Address)
	ioTimeout := time.Duration(c.Timeout) * time.Millisecond
	connectTimeout := ioTimeout
	if deadline, ok := ctx.Deadline(); ok {
		if d := time.Until(deadline); d > 0 && d < connectTimeout {
			connectTimeout = d
		}
	}
	// Use ctx-derived deadline only for Connect; keep session I/O on configured timeout.
	handler.Timeout = connectTimeout
	handler.SlaveId = c.SlaveID

	errCh := make(chan error, 1)
	go func() {
		errCh <- handler.Connect()
	}()
	select {
	case <-ctx.Done():
		_ = handler.Close()
		return nil, fmt.Errorf("modbus_tcp connect: %w", ctx.Err())
	case err := <-errCh:
		if err != nil {
			return nil, fmt.Errorf("modbus_tcp connect: %w", err)
		}
	}
	handler.Timeout = ioTimeout
	client := modbus.NewClient(handler)
	return &session{handler: handler, client: client}, nil
}

type session struct {
	handler *modbus.TCPClientHandler
	client  modbus.Client
}

func (s *session) Read(ctx context.Context, points []plugin.PointAddr) ([]plugin.Sample, error) {
	now := time.Now().UTC()
	out := make([]plugin.Sample, 0, len(points))
	for _, p := range points {
		select {
		case <-ctx.Done():
			return out, ctx.Err()
		default:
		}
		var pa pointAddr
		if err := json.Unmarshal(p.Address, &pa); err != nil {
			out = append(out, plugin.Sample{PointID: p.ID, Timestamp: now, Quality: plugin.QualityBad, Value: nil})
			continue
		}
		if pa.Quantity == 0 {
			pa.Quantity = 1
		}
		if pa.DataType == "" {
			pa.DataType = p.DataType
		}
		if pa.DataType == "" {
			pa.DataType = "uint16"
		}
		val, addrStr, err := s.readOne(pa)
		sm := plugin.Sample{
			PointID:   p.ID,
			Address:   addrStr,
			Timestamp: now,
			Value:     val,
			Quality:   plugin.QualityGood,
		}
		if err != nil {
			sm.Quality = plugin.QualityBad
			sm.Value = nil
		}
		out = append(out, sm)
	}
	return out, nil
}

func (s *session) readOne(pa pointAddr) (any, string, error) {
	addrStr := fmt.Sprintf("%s:%d", pa.RegisterType, pa.Address)
	var results []byte
	var err error
	switch pa.RegisterType {
	case "holding", "":
		results, err = s.client.ReadHoldingRegisters(pa.Address, pa.Quantity)
	case "input":
		results, err = s.client.ReadInputRegisters(pa.Address, pa.Quantity)
	case "coil":
		results, err = s.client.ReadCoils(pa.Address, pa.Quantity)
	case "discrete":
		results, err = s.client.ReadDiscreteInputs(pa.Address, pa.Quantity)
	default:
		return nil, addrStr, fmt.Errorf("unknown registerType %q", pa.RegisterType)
	}
	if err != nil {
		return nil, addrStr, err
	}
	val, err := decode(results, pa.DataType, pa.ByteOrder)
	return val, addrStr, err
}

func decode(b []byte, dataType, order string) (any, error) {
	switch dataType {
	case "bool":
		if len(b) == 0 {
			return false, nil
		}
		return b[0]&0x01 == 1, nil
	case "uint16", "":
		if len(b) < 2 {
			return nil, fmt.Errorf("short buffer")
		}
		return binary.BigEndian.Uint16(b), nil
	case "int16":
		if len(b) < 2 {
			return nil, fmt.Errorf("short buffer")
		}
		return int16(binary.BigEndian.Uint16(b)), nil
	case "uint32", "float32":
		if len(b) < 4 {
			return nil, fmt.Errorf("short buffer")
		}
		u := rearrange32(b, order)
		if dataType == "float32" {
			return math.Float32frombits(u), nil
		}
		return u, nil
	default:
		return nil, fmt.Errorf("unsupported dataType %q", dataType)
	}
}

func rearrange32(b []byte, order string) uint32 {
	switch order {
	case "cdab":
		return binary.BigEndian.Uint32([]byte{b[2], b[3], b[0], b[1]})
	case "badc":
		return binary.BigEndian.Uint32([]byte{b[1], b[0], b[3], b[2]})
	case "dcba":
		return binary.BigEndian.Uint32([]byte{b[3], b[2], b[1], b[0]})
	default: // abcd / big
		return binary.BigEndian.Uint32(b)
	}
}

func (s *session) Subscribe(ctx context.Context, points []plugin.PointAddr, sink chan<- plugin.Sample) (plugin.Subscription, error) {
	stop := make(chan struct{})
	go func() {
		t := time.NewTicker(time.Second)
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
	return s.handler.Close()
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
