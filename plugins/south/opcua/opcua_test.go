package opcua_test

import (
	"encoding/json"
	"testing"

	"github.com/ccc112233/dag/plugins/south/opcua"
)

func TestValidateConfig(t *testing.T) {
	d := opcua.Driver{}
	if err := d.ValidateConfig(json.RawMessage(`{}`)); err == nil {
		t.Fatal("expected error for missing endpoint")
	}
	if err := d.ValidateConfig(json.RawMessage(`{"endpoint":"opc.tcp://127.0.0.1:4840","insecure":true}`)); err != nil {
		t.Fatal(err)
	}
}
