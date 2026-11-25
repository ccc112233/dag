package api_test

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/ccc112233/dag/internal/api"
	"github.com/ccc112233/dag/internal/pipeline"
	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/internal/store"
	northstub "github.com/ccc112233/dag/plugins/north/stub"
	southstub "github.com/ccc112233/dag/plugins/south/stub"
)

func setup(t *testing.T) http.Handler {
	t.Helper()
	st, err := store.Open(filepath.Join(t.TempDir(), "api.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = st.Close() })
	reg := plugin.NewRegistry()
	reg.RegisterSouth(southstub.Driver{})
	reg.RegisterNorth(northstub.Driver{})
	eng := pipeline.New(st, reg, nil, 100)
	t.Cleanup(eng.Close)
	return (&api.Server{Store: st, Registry: reg, Engine: eng, Token: "secret"}).Handler()
}

func TestAuthAndModelVersion(t *testing.T) {
	h := setup(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/plugins", nil)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("want 401 got %d", rec.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz %d", rec.Code)
	}

	body := `{"modelId":"pump","version":"1.0.0","name":"Pump","properties":[{"id":"flow","name":"Flow","dataType":"float"},{"id":"press","name":"P","dataType":"float"},{"id":"temp","name":"T","dataType":"float"}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/models", bytes.NewBufferString(body))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create model %d %s", rec.Code, rec.Body.String())
	}

	body2 := `{"version":"2.0.0","name":"Pump","properties":[{"id":"flow","name":"Flow","dataType":"float"},{"id":"press","name":"P","dataType":"float"},{"id":"temp","name":"T","dataType":"float"},{"id":"alarm","name":"A","dataType":"bool"}]}`
	req = httptest.NewRequest(http.MethodPost, "/api/v1/models/pump/versions", bytes.NewBufferString(body2))
	req.Header.Set("Authorization", "Bearer secret")
	req.Header.Set("Content-Type", "application/json")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create v2 %d %s", rec.Code, rec.Body.String())
	}

	req = httptest.NewRequest(http.MethodGet, "/api/v1/models/pump/versions", nil)
	req.Header.Set("Authorization", "Bearer secret")
	rec = httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list versions %d", rec.Code)
	}
	var versions []map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &versions)
	if len(versions) != 2 {
		t.Fatalf("want 2 versions got %d", len(versions))
	}
}
