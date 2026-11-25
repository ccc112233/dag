package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/ccc112233/dag/internal/pipeline"
	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/internal/store"
	"github.com/ccc112233/dag/internal/tscache"
	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// Server is the management HTTP API.
type Server struct {
	Store    *store.Store
	Registry *plugin.Registry
	Engine   *pipeline.Engine
	Token    string
	TS       *tscache.Cache
}

// Handler returns the root http.Handler.
func (s *Server) Handler() http.Handler {
	r := chi.NewRouter()
	r.Get("/healthz", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ok"})
	})
	r.Get("/readyz", func(w http.ResponseWriter, r *http.Request) {
		if err := s.Store.Ping(r.Context()); err != nil {
			writeErr(w, http.StatusServiceUnavailable, err.Error())
			return
		}
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	r.Route("/api/v1", func(r chi.Router) {
		r.Use(s.auth)
		r.Get("/plugins", s.listPlugins)

		r.Get("/south/connections", s.listSouth)
		r.Post("/south/connections", s.createSouth)
		r.Get("/south/connections/{id}", s.getSouth)
		r.Put("/south/connections/{id}", s.putSouth)
		r.Delete("/south/connections/{id}", s.deleteSouth)

		r.Post("/south/points", s.createPoint)
		r.Get("/south/connections/{id}/points", s.listPoints)

		r.Get("/south/tasks", s.listTasks)
		r.Post("/south/tasks", s.createTask)
		r.Get("/south/tasks/{id}", s.getTask)
		r.Put("/south/tasks/{id}", s.putTask)
		r.Delete("/south/tasks/{id}", s.deleteTask)
		r.Post("/south/tasks/{id}/start", s.startTask)
		r.Post("/south/tasks/{id}/stop", s.stopTask)
		r.Get("/south/tasks/{id}/metrics", s.taskMetrics)

		r.Get("/models", s.listModels)
		r.Post("/models", s.createModel)
		r.Get("/models/{id}", s.getModel)
		r.Get("/models/{modelId}/versions", s.listModelVersions)
		r.Post("/models/{modelId}/versions", s.createModelVersion)

		r.Get("/devices", s.listDevices)
		r.Post("/devices", s.createDevice)
		r.Get("/devices/{id}", s.getDevice)

		r.Get("/mappings", s.listMappings)
		r.Post("/mappings", s.createMapping)
		r.Delete("/mappings/{id}", s.deleteMapping)

		r.Get("/north/channels", s.listNorth)
		r.Post("/north/channels", s.createNorth)
		r.Get("/north/channels/{id}", s.getNorth)
		r.Put("/north/channels/{id}", s.putNorth)
		r.Delete("/north/channels/{id}", s.deleteNorth)
		r.Post("/north/channels/{id}/bindings", s.bindNorth)
		r.Get("/north/channels/{id}/metrics", s.northMetrics)

		r.Get("/ts/stats", s.tsStats)
		r.Get("/ts/query", s.tsQuery)
	})
	return r
}

func (s *Server) tsStats(w http.ResponseWriter, _ *http.Request) {
	if s.TS == nil {
		writeErr(w, http.StatusServiceUnavailable, "ts cache disabled")
		return
	}
	written, dropped := s.TS.Stats()
	writeJSON(w, http.StatusOK, map[string]any{
		"path":          s.TS.Dir(),
		"retentionDays": int(s.TS.Retention() / (24 * time.Hour)),
		"written":       written,
		"dropped":       dropped,
	})
}

func (s *Server) tsQuery(w http.ResponseWriter, r *http.Request) {
	if s.TS == nil {
		writeErr(w, http.StatusServiceUnavailable, "ts cache disabled")
		return
	}
	pointID := r.URL.Query().Get("pointId")
	if pointID == "" {
		writeErr(w, http.StatusBadRequest, "pointId required")
		return
	}
	var from, to time.Time
	if v := r.URL.Query().Get("from"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			t, err = time.Parse(time.RFC3339, v)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid from")
			return
		}
		from = t
	}
	if v := r.URL.Query().Get("to"); v != "" {
		t, err := time.Parse(time.RFC3339Nano, v)
		if err != nil {
			t, err = time.Parse(time.RFC3339, v)
		}
		if err != nil {
			writeErr(w, http.StatusBadRequest, "invalid to")
			return
		}
		to = t
	}
	limit := 1000
	if v := r.URL.Query().Get("limit"); v != "" {
		n, err := strconv.Atoi(v)
		if err != nil || n <= 0 {
			writeErr(w, http.StatusBadRequest, "invalid limit")
			return
		}
		limit = n
	}
	samples, err := s.TS.Query(pointID, from, to, limit)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"pointId": pointID, "samples": samples})
}

func (s *Server) auth(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := r.Header.Get("Authorization")
		if !strings.HasPrefix(h, "Bearer ") || strings.TrimPrefix(h, "Bearer ") != s.Token {
			writeErr(w, http.StatusUnauthorized, "unauthorized")
			return
		}
		next.ServeHTTP(w, r)
	})
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func writeErr(w http.ResponseWriter, code int, msg string) {
	writeJSON(w, code, map[string]string{"error": msg})
}

func decode(r *http.Request, v any) error {
	defer r.Body.Close()
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	return dec.Decode(v)
}

func (s *Server) listPlugins(w http.ResponseWriter, _ *http.Request) {
	south, north := s.Registry.List()
	writeJSON(w, http.StatusOK, map[string]any{"south": south, "north": north})
}

func (s *Server) listSouth(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSouthConnections(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createSouth(w http.ResponseWriter, r *http.Request) {
	var c store.SouthConnection
	if err := decode(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	if c.DriverType == "" {
		writeErr(w, http.StatusBadRequest, "driverType required")
		return
	}
	d, err := s.Registry.South(c.DriverType)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := d.ValidateConfig(c.EndpointJSON); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c.CreatedAt = time.Now().UTC()
	if err := s.Store.UpsertSouthConnection(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) getSouth(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetSouthConnection(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) putSouth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var c store.SouthConnection
	if err := decode(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c.ID = id
	if c.DriverType == "" {
		writeErr(w, http.StatusBadRequest, "driverType required")
		return
	}
	d, err := s.Registry.South(c.DriverType)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := d.ValidateConfig(c.EndpointJSON); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpsertSouthConnection(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteSouth(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSouthConnection(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) createPoint(w http.ResponseWriter, r *http.Request) {
	var p store.Point
	if err := decode(r, &p); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if p.ID == "" {
		p.ID = uuid.NewString()
	}
	if err := s.Store.UpsertPoint(r.Context(), p); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, p)
}

func (s *Server) listPoints(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListPointsByConnection(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListCollectTasks(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var t store.CollectTask
	if err := decode(r, &t); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if t.ID == "" {
		t.ID = uuid.NewString()
	}
	if t.Mode == "" {
		t.Mode = "poll"
	}
	if err := s.Store.UpsertCollectTask(r.Context(), t); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, t)
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	t, err := s.Store.GetCollectTask(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) putTask(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var t store.CollectTask
	if err := decode(r, &t); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	t.ID = id
	if err := s.Store.UpsertCollectTask(r.Context(), t); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, t)
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	_ = s.Engine.StopTask(r.Context(), chi.URLParam(r, "id"))
	if err := s.Store.DeleteCollectTask(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) startTask(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.StartTask(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "started"})
}

func (s *Server) stopTask(w http.ResponseWriter, r *http.Request) {
	if err := s.Engine.StopTask(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "stopped"})
}

func (s *Server) taskMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := s.Store.GetMetric(r.Context(), "task", chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) listModels(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListThingModels(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createModel(w http.ResponseWriter, r *http.Request) {
	var m store.ThingModel
	if err := decode(r, &m); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ModelID == "" || m.Version == "" {
		writeErr(w, http.StatusBadRequest, "modelId and version required")
		return
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if existing, err := s.Store.GetThingModel(r.Context(), m.ModelID, m.Version); err == nil && existing != nil {
		writeErr(w, http.StatusConflict, "model version already exists")
		return
	}
	m.CreatedAt = time.Now().UTC()
	if err := s.Store.UpsertThingModel(r.Context(), m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) getModel(w http.ResponseWriter, r *http.Request) {
	m, err := s.Store.GetThingModelByPK(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func (s *Server) listModelVersions(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListModelVersions(r.Context(), chi.URLParam(r, "modelId"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createModelVersion(w http.ResponseWriter, r *http.Request) {
	modelID := chi.URLParam(r, "modelId")
	var m store.ThingModel
	if err := decode(r, &m); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	m.ModelID = modelID
	if m.Version == "" {
		writeErr(w, http.StatusBadRequest, "version required")
		return
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if existing, err := s.Store.GetThingModel(r.Context(), m.ModelID, m.Version); err == nil && existing != nil {
		writeErr(w, http.StatusConflict, "model version already exists")
		return
	}
	m.CreatedAt = time.Now().UTC()
	if err := s.Store.UpsertThingModel(r.Context(), m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) listDevices(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListDevices(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createDevice(w http.ResponseWriter, r *http.Request) {
	var d store.Device
	if err := decode(r, &d); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if d.DeviceID == "" || d.ModelID == "" || d.ModelVersion == "" {
		writeErr(w, http.StatusBadRequest, "deviceId, modelId, modelVersion required")
		return
	}
	if _, err := s.Store.GetThingModel(r.Context(), d.ModelID, d.ModelVersion); err != nil {
		writeErr(w, http.StatusBadRequest, "model version not found")
		return
	}
	if d.ID == "" {
		d.ID = uuid.NewString()
	}
	if err := s.Store.UpsertDevice(r.Context(), d); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, d)
}

func (s *Server) getDevice(w http.ResponseWriter, r *http.Request) {
	d, err := s.Store.GetDevice(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, d)
}

func (s *Server) listMappings(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListMappings(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createMapping(w http.ResponseWriter, r *http.Request) {
	var m store.Mapping
	if err := decode(r, &m); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if m.ID == "" {
		m.ID = uuid.NewString()
	}
	if m.Scale == 0 {
		m.Scale = 1
	}
	if err := s.Store.UpsertMapping(r.Context(), m); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, m)
}

func (s *Server) deleteMapping(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteMapping(r.Context(), chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listNorth(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListNorthChannels(r.Context())
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (s *Server) createNorth(w http.ResponseWriter, r *http.Request) {
	var c store.NorthChannel
	if err := decode(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if c.ID == "" {
		c.ID = uuid.NewString()
	}
	d, err := s.Registry.North(c.ChannelType)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := d.ValidateConfig(c.ConfigJSON); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpsertNorthChannel(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, c)
}

func (s *Server) getNorth(w http.ResponseWriter, r *http.Request) {
	c, err := s.Store.GetNorthChannel(r.Context(), chi.URLParam(r, "id"))
	if err != nil {
		mapStoreErr(w, err)
		return
	}
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) putNorth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	var c store.NorthChannel
	if err := decode(r, &c); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	c.ID = id
	d, err := s.Registry.North(c.ChannelType)
	if err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := d.ValidateConfig(c.ConfigJSON); err != nil {
		writeErr(w, http.StatusBadRequest, err.Error())
		return
	}
	if err := s.Store.UpsertNorthChannel(r.Context(), c); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	s.Engine.InvalidatePublisher(id)
	writeJSON(w, http.StatusOK, c)
}

func (s *Server) deleteNorth(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	s.Engine.InvalidatePublisher(id)
	if err := s.Store.DeleteNorthChannel(r.Context(), id); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) bindNorth(w http.ResponseWriter, r *http.Request) {
	var body struct {
		DeviceID string `json:"deviceId"`
	}
	if err := decode(r, &body); err != nil || body.DeviceID == "" {
		writeErr(w, http.StatusBadRequest, "deviceId required")
		return
	}
	if err := s.Store.BindChannelDevice(r.Context(), chi.URLParam(r, "id"), body.DeviceID); err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, map[string]string{"status": "bound"})
}

func (s *Server) northMetrics(w http.ResponseWriter, r *http.Request) {
	m, err := s.Store.GetMetric(r.Context(), "channel", chi.URLParam(r, "id"))
	if err != nil {
		writeErr(w, http.StatusInternalServerError, err.Error())
		return
	}
	writeJSON(w, http.StatusOK, m)
}

func mapStoreErr(w http.ResponseWriter, err error) {
	if errors.Is(err, sql.ErrNoRows) || store.IsNotFound(err) {
		writeErr(w, http.StatusNotFound, "not found")
		return
	}
	writeErr(w, http.StatusInternalServerError, err.Error())
}
