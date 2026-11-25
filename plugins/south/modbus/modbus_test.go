package modbus_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"net"
	"testing"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	mb "github.com/ccc112233/dag/plugins/south/modbus"
)

// minimal Modbus TCP server answering Read Holding Registers (FC 0x03) with fixed uint16=42.
func startMockModbus(t *testing.T) string {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = ln.Close() })
	go func() {
		for {
			conn, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				buf := make([]byte, 256)
				for {
					_ = c.SetReadDeadline(time.Now().Add(2 * time.Second))
					n, err := c.Read(buf)
					if err != nil || n < 12 {
						return
					}
					// MBAP + PDU
					tx := binary.BigEndian.Uint16(buf[0:2])
					unit := buf[6]
					fc := buf[7]
					resp := make([]byte, 0, 32)
					resp = append(resp, buf[0:2]...) // tx
					resp = append(resp, 0, 0)         // proto
					if fc == 0x03 {
						pdu := []byte{unit, 0x03, 0x02, 0x00, 0x2a} // value 42
						binary.BigEndian.PutUint16(resp[0:2], tx)
						lenBuf := make([]byte, 2)
						binary.BigEndian.PutUint16(lenBuf, uint16(len(pdu)))
						frame := append([]byte{}, buf[0:2]...)
						frame = append(frame, 0, 0)
						frame = append(frame, lenBuf...)
						frame = append(frame, pdu...)
						_, _ = c.Write(frame)
					} else {
						_ = resp
					}
				}
			}(conn)
		}
	}()
	return ln.Addr().String()
}

func TestModbusTCPRead(t *testing.T) {
	addr := startMockModbus(t)
	d := mb.Driver{}
	cfg, _ := json.Marshal(map[string]any{"address": addr, "slaveId": 1, "timeoutMs": 2000})
	if err := d.ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	sess, err := d.Open(ctx, cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer sess.Close()
	pa, _ := json.Marshal(map[string]any{"registerType": "holding", "address": 0, "quantity": 1, "dataType": "uint16"})
	samples, err := sess.Read(ctx, []plugin.PointAddr{{ID: "p1", Address: pa}})
	if err != nil {
		t.Fatal(err)
	}
	if len(samples) != 1 || samples[0].Quality != plugin.QualityGood {
		t.Fatalf("samples=%+v", samples)
	}
	v, ok := samples[0].Value.(uint16)
	if !ok || v != 42 {
		t.Fatalf("value=%#v", samples[0].Value)
	}
}

func TestOpenRespectsContext(t *testing.T) {
	addr := startMockModbus(t)
	cfg, _ := json.Marshal(map[string]any{"address": addr, "slaveId": 1, "timeoutMs": 30000})
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := mb.Driver{}.Open(ctx, cfg, nil)
	if err == nil {
		t.Fatal("expected already-cancelled context to fail Open")
	}
}
