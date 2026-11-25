package amqp_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ccc112233/dag/plugins/north/amqp"
)

func TestValidateAndOptionalBroker(t *testing.T) {
	d := amqp.Driver{}
	if err := d.ValidateConfig(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected validation error")
	}
	cfg := json.RawMessage(`{"url":"amqp://guest:guest@127.0.0.1:5672/","queue":"dap.telemetry"}`)
	if err := d.ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ACCEPTANCE_EXTERNAL") != "1" {
		t.Skip("set ACCEPTANCE_EXTERNAL=1 with rabbitmq up for live publish")
	}
}
