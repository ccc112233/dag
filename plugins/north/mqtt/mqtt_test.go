package mqtt_test

import (
	"encoding/json"
	"os"
	"testing"

	"github.com/ccc112233/dag/plugins/north/mqtt"
)

func TestValidateAndOptionalBroker(t *testing.T) {
	d := mqtt.Driver{}
	if err := d.ValidateConfig(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected validation error")
	}
	cfg := json.RawMessage(`{"broker":"tcp://127.0.0.1:1883","topic":"dap/{deviceId}","qos":0}`)
	if err := d.ValidateConfig(cfg); err != nil {
		t.Fatal(err)
	}
	if os.Getenv("ACCEPTANCE_EXTERNAL") != "1" {
		t.Skip("set ACCEPTANCE_EXTERNAL=1 with mosquitto up for live publish")
	}
}
