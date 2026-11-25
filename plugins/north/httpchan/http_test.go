package httpchan_test

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/plugins/north/httpchan"
)

func TestHTTPPublish(t *testing.T) {
	var gotAuth atomic.Value
	var gotBody atomic.Value
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotAuth.Store(r.Header.Get("Authorization"))
		b, _ := io.ReadAll(r.Body)
		gotBody.Store(string(b))
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	d := httpchan.Driver{}
	cfg, _ := json.Marshal(map[string]any{
		"url": srv.URL, "headers": map[string]string{"Authorization": "Bearer test-token"},
	})
	pub, err := d.Open(context.Background(), cfg, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer pub.Close()
	env := plugin.Envelope{
		MessageID: "m1", DeviceID: "d1", ModelID: "motor", ModelVersion: "1.0.0",
		Timestamp: time.Now().UTC(),
		Properties: map[string]plugin.PropertyValue{"temp": {Value: 1.5, Quality: plugin.QualityGood}},
	}
	if err := pub.Publish(context.Background(), env); err != nil {
		t.Fatal(err)
	}
	if gotAuth.Load().(string) != "Bearer test-token" {
		t.Fatalf("auth=%v", gotAuth.Load())
	}
	var parsed plugin.Envelope
	if err := json.Unmarshal([]byte(gotBody.Load().(string)), &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.DeviceID != "d1" || parsed.Properties["temp"].Value.(float64) != 1.5 {
		t.Fatalf("body=%s", gotBody.Load())
	}
}
