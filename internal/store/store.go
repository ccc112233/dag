package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"time"

	_ "modernc.org/sqlite"
)

// Store wraps SQLite persistence.
type Store struct {
	db *sql.DB
}

// Open opens (or creates) a SQLite database and migrates schema.
func Open(path string) (*Store, error) {
	db, err := sql.Open("sqlite", path+"?_pragma=busy_timeout(5000)&_pragma=journal_mode(WAL)")
	if err != nil {
		return nil, err
	}
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)
	s := &Store{db: db}
	if err := s.migrate(); err != nil {
		_ = db.Close()
		return nil, err
	}
	return s, nil
}

// Close closes the DB.
func (s *Store) Close() error { return s.db.Close() }

// DB exposes the underlying handle for readiness checks.
func (s *Store) DB() *sql.DB { return s.db }

func (s *Store) migrate() error {
	_, err := s.db.Exec(`
CREATE TABLE IF NOT EXISTS south_connection (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  driver_type TEXT NOT NULL,
  endpoint_json TEXT NOT NULL,
  credentials_ref TEXT,
  timeout_ms INTEGER NOT NULL DEFAULT 5000,
  retry_json TEXT,
  status TEXT NOT NULL DEFAULT 'idle',
  created_at TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS point (
  id TEXT PRIMARY KEY,
  connection_id TEXT NOT NULL,
  address_json TEXT NOT NULL,
  data_type TEXT,
  name TEXT,
  FOREIGN KEY(connection_id) REFERENCES south_connection(id)
);
CREATE TABLE IF NOT EXISTS collect_task (
  id TEXT PRIMARY KEY,
  connection_id TEXT NOT NULL,
  mode TEXT NOT NULL,
  interval_ms INTEGER NOT NULL DEFAULT 1000,
  point_set_json TEXT NOT NULL,
  enabled INTEGER NOT NULL DEFAULT 0,
  FOREIGN KEY(connection_id) REFERENCES south_connection(id)
);
CREATE TABLE IF NOT EXISTS thing_model (
  id TEXT PRIMARY KEY,
  model_id TEXT NOT NULL,
  version TEXT NOT NULL,
  name TEXT NOT NULL,
  properties_json TEXT NOT NULL,
  services_json TEXT,
  events_json TEXT,
  created_at TEXT NOT NULL,
  UNIQUE(model_id, version)
);
CREATE TABLE IF NOT EXISTS device (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL UNIQUE,
  model_id TEXT NOT NULL,
  model_version TEXT NOT NULL,
  name TEXT NOT NULL
);
CREATE TABLE IF NOT EXISTS mapping (
  id TEXT PRIMARY KEY,
  device_id TEXT NOT NULL,
  point_id TEXT NOT NULL,
  property_id TEXT NOT NULL,
  scale REAL NOT NULL DEFAULT 1,
  offset REAL NOT NULL DEFAULT 0,
  unit TEXT
);
CREATE TABLE IF NOT EXISTS north_channel (
  id TEXT PRIMARY KEY,
  name TEXT NOT NULL,
  channel_type TEXT NOT NULL,
  config_json TEXT NOT NULL,
  credentials_ref TEXT,
  enabled INTEGER NOT NULL DEFAULT 0,
  trigger TEXT NOT NULL DEFAULT 'change',
  interval_ms INTEGER NOT NULL DEFAULT 1000
);
CREATE TABLE IF NOT EXISTS channel_binding (
  channel_id TEXT NOT NULL,
  device_id TEXT NOT NULL,
  PRIMARY KEY(channel_id, device_id)
);
CREATE TABLE IF NOT EXISTS runtime_metric (
  subject_type TEXT NOT NULL,
  subject_id TEXT NOT NULL,
  success_count INTEGER NOT NULL DEFAULT 0,
  fail_count INTEGER NOT NULL DEFAULT 0,
  last_error TEXT,
  last_sample_at TEXT,
  PRIMARY KEY(subject_type, subject_id)
);
CREATE TABLE IF NOT EXISTS outbound_buffer (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  channel_id TEXT NOT NULL,
  seq INTEGER NOT NULL,
  payload_json TEXT NOT NULL,
  created_at TEXT NOT NULL,
  attempts INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX IF NOT EXISTS idx_outbound_channel_seq ON outbound_buffer(channel_id, seq);
`)
	return err
}

// --- types ---

type SouthConnection struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	DriverType     string          `json:"driverType"`
	EndpointJSON   json.RawMessage `json:"endpoint"`
	CredentialsRef string          `json:"credentialsRef,omitempty"`
	TimeoutMs      int             `json:"timeoutMs"`
	RetryJSON      json.RawMessage `json:"retry,omitempty"`
	Status         string          `json:"status"`
	CreatedAt      time.Time       `json:"createdAt"`
}

type Point struct {
	ID           string          `json:"id"`
	ConnectionID string          `json:"connectionId"`
	AddressJSON  json.RawMessage `json:"address"`
	DataType     string          `json:"dataType,omitempty"`
	Name         string          `json:"name,omitempty"`
}

type CollectTask struct {
	ID           string          `json:"id"`
	ConnectionID string          `json:"connectionId"`
	Mode         string          `json:"mode"`
	IntervalMs   int             `json:"intervalMs"`
	PointSetJSON json.RawMessage `json:"pointSet"`
	Enabled      bool            `json:"enabled"`
}

type PropertyDef struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	DataType    string `json:"dataType"`
	Unit        string `json:"unit,omitempty"`
	Description string `json:"description,omitempty"`
	Writable    bool   `json:"writable,omitempty"`
}

type ThingModel struct {
	ID           string          `json:"id"`
	ModelID      string          `json:"modelId"`
	Version      string          `json:"version"`
	Name         string          `json:"name"`
	Properties   []PropertyDef   `json:"properties"`
	ServicesJSON json.RawMessage `json:"services,omitempty"`
	EventsJSON   json.RawMessage `json:"events,omitempty"`
	CreatedAt    time.Time       `json:"createdAt"`
}

type Device struct {
	ID           string `json:"id"`
	DeviceID     string `json:"deviceId"`
	ModelID      string `json:"modelId"`
	ModelVersion string `json:"modelVersion"`
	Name         string `json:"name"`
}

type Mapping struct {
	ID         string  `json:"id"`
	DeviceID   string  `json:"deviceId"`
	PointID    string  `json:"pointId"`
	PropertyID string  `json:"propertyId"`
	Scale      float64 `json:"scale"`
	Offset     float64 `json:"offset"`
	Unit       string  `json:"unit,omitempty"`
}

type NorthChannel struct {
	ID             string          `json:"id"`
	Name           string          `json:"name"`
	ChannelType    string          `json:"channelType"`
	ConfigJSON     json.RawMessage `json:"config"`
	CredentialsRef string          `json:"credentialsRef,omitempty"`
	Enabled        bool            `json:"enabled"`
	Trigger        string          `json:"trigger"`
	IntervalMs     int             `json:"intervalMs"`
}

type RuntimeMetric struct {
	SubjectType   string     `json:"subjectType"`
	SubjectID     string     `json:"subjectId"`
	SuccessCount  int64      `json:"successCount"`
	FailCount     int64      `json:"failCount"`
	LastError     string     `json:"lastError,omitempty"`
	LastSampleAt  *time.Time `json:"lastSampleAt,omitempty"`
}

type BufferItem struct {
	ID        int64
	ChannelID string
	Seq       int64
	Payload   []byte
	CreatedAt time.Time
	Attempts  int
}

func nowRFC() string { return time.Now().UTC().Format(time.RFC3339Nano) }

// --- CRUD helpers (selected) ---

func (s *Store) UpsertSouthConnection(ctx context.Context, c SouthConnection) error {
	if c.CreatedAt.IsZero() {
		c.CreatedAt = time.Now().UTC()
	}
	if c.Status == "" {
		c.Status = "idle"
	}
	if c.TimeoutMs == 0 {
		c.TimeoutMs = 5000
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO south_connection(id,name,driver_type,endpoint_json,credentials_ref,timeout_ms,retry_json,status,created_at)
VALUES(?,?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET
 name=excluded.name, driver_type=excluded.driver_type, endpoint_json=excluded.endpoint_json,
 credentials_ref=excluded.credentials_ref, timeout_ms=excluded.timeout_ms, retry_json=excluded.retry_json, status=excluded.status`,
		c.ID, c.Name, c.DriverType, string(c.EndpointJSON), c.CredentialsRef, c.TimeoutMs, nullBytes(c.RetryJSON), c.Status, c.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetSouthConnection(ctx context.Context, id string) (*SouthConnection, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,driver_type,endpoint_json,credentials_ref,timeout_ms,retry_json,status,created_at FROM south_connection WHERE id=?`, id)
	var c SouthConnection
	var endpoint, created string
	var cred, retry sql.NullString
	if err := row.Scan(&c.ID, &c.Name, &c.DriverType, &endpoint, &cred, &c.TimeoutMs, &retry, &c.Status, &created); err != nil {
		return nil, err
	}
	c.EndpointJSON = json.RawMessage(endpoint)
	if cred.Valid {
		c.CredentialsRef = cred.String
	}
	if retry.Valid && retry.String != "" {
		c.RetryJSON = json.RawMessage(retry.String)
	}
	c.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &c, nil
}

func (s *Store) ListSouthConnections(ctx context.Context) ([]SouthConnection, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM south_connection ORDER BY created_at`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []SouthConnection
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		c, err := s.GetSouthConnection(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteSouthConnection(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM south_connection WHERE id=?`, id)
	return err
}

func (s *Store) UpsertPoint(ctx context.Context, p Point) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO point(id,connection_id,address_json,data_type,name) VALUES(?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET connection_id=excluded.connection_id, address_json=excluded.address_json, data_type=excluded.data_type, name=excluded.name`,
		p.ID, p.ConnectionID, string(p.AddressJSON), p.DataType, p.Name)
	return err
}

func (s *Store) GetPoint(ctx context.Context, id string) (*Point, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,connection_id,address_json,data_type,name FROM point WHERE id=?`, id)
	var p Point
	var addr string
	var dt, name sql.NullString
	if err := row.Scan(&p.ID, &p.ConnectionID, &addr, &dt, &name); err != nil {
		return nil, err
	}
	p.AddressJSON = json.RawMessage(addr)
	if dt.Valid {
		p.DataType = dt.String
	}
	if name.Valid {
		p.Name = name.String
	}
	return &p, nil
}

func (s *Store) ListPointsByConnection(ctx context.Context, connID string) ([]Point, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM point WHERE connection_id=?`, connID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Point
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		p, err := s.GetPoint(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *p)
	}
	return out, rows.Err()
}

func (s *Store) UpsertCollectTask(ctx context.Context, t CollectTask) error {
	en := 0
	if t.Enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO collect_task(id,connection_id,mode,interval_ms,point_set_json,enabled) VALUES(?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET connection_id=excluded.connection_id, mode=excluded.mode, interval_ms=excluded.interval_ms, point_set_json=excluded.point_set_json, enabled=excluded.enabled`,
		t.ID, t.ConnectionID, t.Mode, t.IntervalMs, string(t.PointSetJSON), en)
	return err
}

func (s *Store) GetCollectTask(ctx context.Context, id string) (*CollectTask, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,connection_id,mode,interval_ms,point_set_json,enabled FROM collect_task WHERE id=?`, id)
	var t CollectTask
	var ps string
	var en int
	if err := row.Scan(&t.ID, &t.ConnectionID, &t.Mode, &t.IntervalMs, &ps, &en); err != nil {
		return nil, err
	}
	t.PointSetJSON = json.RawMessage(ps)
	t.Enabled = en == 1
	return &t, nil
}

func (s *Store) ListCollectTasks(ctx context.Context) ([]CollectTask, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM collect_task`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []CollectTask
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		t, err := s.GetCollectTask(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *t)
	}
	return out, rows.Err()
}

func (s *Store) SetTaskEnabled(ctx context.Context, id string, enabled bool) error {
	en := 0
	if enabled {
		en = 1
	}
	_, err := s.db.ExecContext(ctx, `UPDATE collect_task SET enabled=? WHERE id=?`, en, id)
	return err
}

func (s *Store) DeleteCollectTask(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM collect_task WHERE id=?`, id)
	return err
}

func (s *Store) UpsertThingModel(ctx context.Context, m ThingModel) error {
	if m.CreatedAt.IsZero() {
		m.CreatedAt = time.Now().UTC()
	}
	props, err := json.Marshal(m.Properties)
	if err != nil {
		return err
	}
	_, err = s.db.ExecContext(ctx, `
INSERT INTO thing_model(id,model_id,version,name,properties_json,services_json,events_json,created_at)
VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, properties_json=excluded.properties_json, services_json=excluded.services_json, events_json=excluded.events_json`,
		m.ID, m.ModelID, m.Version, m.Name, string(props), nullBytes(m.ServicesJSON), nullBytes(m.EventsJSON), m.CreatedAt.Format(time.RFC3339Nano))
	return err
}

func (s *Store) GetThingModelByPK(ctx context.Context, id string) (*ThingModel, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,model_id,version,name,properties_json,services_json,events_json,created_at FROM thing_model WHERE id=?`, id)
	return scanThingModel(row)
}

func (s *Store) GetThingModel(ctx context.Context, modelID, version string) (*ThingModel, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,model_id,version,name,properties_json,services_json,events_json,created_at FROM thing_model WHERE model_id=? AND version=?`, modelID, version)
	return scanThingModel(row)
}

func scanThingModel(row *sql.Row) (*ThingModel, error) {
	var m ThingModel
	var props, created string
	var services, events sql.NullString
	if err := row.Scan(&m.ID, &m.ModelID, &m.Version, &m.Name, &props, &services, &events, &created); err != nil {
		return nil, err
	}
	if err := json.Unmarshal([]byte(props), &m.Properties); err != nil {
		return nil, err
	}
	if services.Valid {
		m.ServicesJSON = json.RawMessage(services.String)
	}
	if events.Valid {
		m.EventsJSON = json.RawMessage(events.String)
	}
	m.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
	return &m, nil
}

func (s *Store) ListThingModels(ctx context.Context) ([]ThingModel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM thing_model ORDER BY model_id, version`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThingModel
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m, err := s.GetThingModelByPK(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) ListModelVersions(ctx context.Context, modelID string) ([]ThingModel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM thing_model WHERE model_id=? ORDER BY created_at`, modelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ThingModel
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		m, err := s.GetThingModelByPK(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *m)
	}
	return out, rows.Err()
}

func (s *Store) UpsertDevice(ctx context.Context, d Device) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO device(id,device_id,model_id,model_version,name) VALUES(?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET device_id=excluded.device_id, model_id=excluded.model_id, model_version=excluded.model_version, name=excluded.name`,
		d.ID, d.DeviceID, d.ModelID, d.ModelVersion, d.Name)
	return err
}

func (s *Store) GetDevice(ctx context.Context, id string) (*Device, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,device_id,model_id,model_version,name FROM device WHERE id=?`, id)
	var d Device
	if err := row.Scan(&d.ID, &d.DeviceID, &d.ModelID, &d.ModelVersion, &d.Name); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) GetDeviceByDeviceID(ctx context.Context, deviceID string) (*Device, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,device_id,model_id,model_version,name FROM device WHERE device_id=?`, deviceID)
	var d Device
	if err := row.Scan(&d.ID, &d.DeviceID, &d.ModelID, &d.ModelVersion, &d.Name); err != nil {
		return nil, err
	}
	return &d, nil
}

func (s *Store) ListDevices(ctx context.Context) ([]Device, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM device`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Device
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		d, err := s.GetDevice(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *d)
	}
	return out, rows.Err()
}

func (s *Store) UpsertMapping(ctx context.Context, m Mapping) error {
	if m.Scale == 0 {
		m.Scale = 1
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO mapping(id,device_id,point_id,property_id,scale,offset,unit) VALUES(?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET device_id=excluded.device_id, point_id=excluded.point_id, property_id=excluded.property_id, scale=excluded.scale, offset=excluded.offset, unit=excluded.unit`,
		m.ID, m.DeviceID, m.PointID, m.PropertyID, m.Scale, m.Offset, m.Unit)
	return err
}

func (s *Store) ListMappings(ctx context.Context) ([]Mapping, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,device_id,point_id,property_id,scale,offset,unit FROM mapping`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		var unit sql.NullString
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.PointID, &m.PropertyID, &m.Scale, &m.Offset, &unit); err != nil {
			return nil, err
		}
		if unit.Valid {
			m.Unit = unit.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) ListMappingsByDevice(ctx context.Context, deviceID string) ([]Mapping, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,device_id,point_id,property_id,scale,offset,unit FROM mapping WHERE device_id=?`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Mapping
	for rows.Next() {
		var m Mapping
		var unit sql.NullString
		if err := rows.Scan(&m.ID, &m.DeviceID, &m.PointID, &m.PropertyID, &m.Scale, &m.Offset, &unit); err != nil {
			return nil, err
		}
		if unit.Valid {
			m.Unit = unit.String
		}
		out = append(out, m)
	}
	return out, rows.Err()
}

func (s *Store) DeleteMapping(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM mapping WHERE id=?`, id)
	return err
}

func (s *Store) UpsertNorthChannel(ctx context.Context, c NorthChannel) error {
	en := 0
	if c.Enabled {
		en = 1
	}
	if c.Trigger == "" {
		c.Trigger = "change"
	}
	if c.IntervalMs == 0 {
		c.IntervalMs = 1000
	}
	_, err := s.db.ExecContext(ctx, `
INSERT INTO north_channel(id,name,channel_type,config_json,credentials_ref,enabled,trigger,interval_ms) VALUES(?,?,?,?,?,?,?,?)
ON CONFLICT(id) DO UPDATE SET name=excluded.name, channel_type=excluded.channel_type, config_json=excluded.config_json, credentials_ref=excluded.credentials_ref, enabled=excluded.enabled, trigger=excluded.trigger, interval_ms=excluded.interval_ms`,
		c.ID, c.Name, c.ChannelType, string(c.ConfigJSON), c.CredentialsRef, en, c.Trigger, c.IntervalMs)
	return err
}

func (s *Store) GetNorthChannel(ctx context.Context, id string) (*NorthChannel, error) {
	row := s.db.QueryRowContext(ctx, `SELECT id,name,channel_type,config_json,credentials_ref,enabled,trigger,interval_ms FROM north_channel WHERE id=?`, id)
	var c NorthChannel
	var cfg string
	var cred sql.NullString
	var en int
	if err := row.Scan(&c.ID, &c.Name, &c.ChannelType, &cfg, &cred, &en, &c.Trigger, &c.IntervalMs); err != nil {
		return nil, err
	}
	c.ConfigJSON = json.RawMessage(cfg)
	if cred.Valid {
		c.CredentialsRef = cred.String
	}
	c.Enabled = en == 1
	return &c, nil
}

func (s *Store) ListNorthChannels(ctx context.Context) ([]NorthChannel, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id FROM north_channel`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NorthChannel
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		c, err := s.GetNorthChannel(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) DeleteNorthChannel(ctx context.Context, id string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM north_channel WHERE id=?`, id)
	return err
}

func (s *Store) BindChannelDevice(ctx context.Context, channelID, deviceID string) error {
	_, err := s.db.ExecContext(ctx, `INSERT OR IGNORE INTO channel_binding(channel_id, device_id) VALUES(?,?)`, channelID, deviceID)
	return err
}

func (s *Store) ListChannelBindings(ctx context.Context, channelID string) ([]string, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT device_id FROM channel_binding WHERE channel_id=?`, channelID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *Store) ListChannelsForDevice(ctx context.Context, deviceID string) ([]NorthChannel, error) {
	rows, err := s.db.QueryContext(ctx, `
SELECT c.id FROM north_channel c
JOIN channel_binding b ON b.channel_id=c.id
WHERE b.device_id=? AND c.enabled=1`, deviceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []NorthChannel
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		c, err := s.GetNorthChannel(ctx, id)
		if err != nil {
			return nil, err
		}
		out = append(out, *c)
	}
	return out, rows.Err()
}

func (s *Store) IncrMetricSuccess(ctx context.Context, subjectType, subjectID string, sampleAt time.Time) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO runtime_metric(subject_type,subject_id,success_count,fail_count,last_sample_at)
VALUES(?,?,1,0,?)
ON CONFLICT(subject_type,subject_id) DO UPDATE SET
 success_count=success_count+1, last_sample_at=excluded.last_sample_at, last_error=''`,
		subjectType, subjectID, sampleAt.UTC().Format(time.RFC3339Nano))
	return err
}

func (s *Store) IncrMetricFail(ctx context.Context, subjectType, subjectID, lastErr string) error {
	_, err := s.db.ExecContext(ctx, `
INSERT INTO runtime_metric(subject_type,subject_id,success_count,fail_count,last_error)
VALUES(?,?,0,1,?)
ON CONFLICT(subject_type,subject_id) DO UPDATE SET
 fail_count=fail_count+1, last_error=excluded.last_error`,
		subjectType, subjectID, lastErr)
	return err
}

func (s *Store) GetMetric(ctx context.Context, subjectType, subjectID string) (*RuntimeMetric, error) {
	row := s.db.QueryRowContext(ctx, `SELECT subject_type,subject_id,success_count,fail_count,last_error,last_sample_at FROM runtime_metric WHERE subject_type=? AND subject_id=?`, subjectType, subjectID)
	var m RuntimeMetric
	var lastErr, lastSample sql.NullString
	if err := row.Scan(&m.SubjectType, &m.SubjectID, &m.SuccessCount, &m.FailCount, &lastErr, &lastSample); err != nil {
		if err == sql.ErrNoRows {
			return &RuntimeMetric{SubjectType: subjectType, SubjectID: subjectID}, nil
		}
		return nil, err
	}
	if lastErr.Valid {
		m.LastError = lastErr.String
	}
	if lastSample.Valid {
		t, err := time.Parse(time.RFC3339Nano, lastSample.String)
		if err == nil {
			m.LastSampleAt = &t
		}
	}
	return &m, nil
}

func (s *Store) NextBufferSeq(ctx context.Context, channelID string) (int64, error) {
	var seq sql.NullInt64
	err := s.db.QueryRowContext(ctx, `SELECT MAX(seq) FROM outbound_buffer WHERE channel_id=?`, channelID).Scan(&seq)
	if err != nil {
		return 0, err
	}
	if !seq.Valid {
		return 1, nil
	}
	return seq.Int64 + 1, nil
}

func (s *Store) EnqueueBuffer(ctx context.Context, channelID string, seq int64, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO outbound_buffer(channel_id,seq,payload_json,created_at,attempts) VALUES(?,?,?,?,0)`,
		channelID, seq, string(payload), nowRFC())
	return err
}

func (s *Store) ListBuffer(ctx context.Context, channelID string, limit int) ([]BufferItem, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,channel_id,seq,payload_json,created_at,attempts FROM outbound_buffer WHERE channel_id=? ORDER BY seq ASC LIMIT ?`, channelID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []BufferItem
	for rows.Next() {
		var it BufferItem
		var payload, created string
		if err := rows.Scan(&it.ID, &it.ChannelID, &it.Seq, &payload, &created, &it.Attempts); err != nil {
			return nil, err
		}
		it.Payload = []byte(payload)
		it.CreatedAt, _ = time.Parse(time.RFC3339Nano, created)
		out = append(out, it)
	}
	return out, rows.Err()
}

func (s *Store) DeleteBuffer(ctx context.Context, id int64) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM outbound_buffer WHERE id=?`, id)
	return err
}

func (s *Store) BufferCount(ctx context.Context, channelID string) (int, error) {
	var n int
	err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM outbound_buffer WHERE channel_id=?`, channelID).Scan(&n)
	return n, err
}

func (s *Store) DropOldestBuffer(ctx context.Context, channelID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM outbound_buffer WHERE id=(SELECT id FROM outbound_buffer WHERE channel_id=? ORDER BY seq ASC LIMIT 1)`, channelID)
	return err
}

func (s *Store) Ping(ctx context.Context) error {
	return s.db.PingContext(ctx)
}

func nullBytes(b json.RawMessage) any {
	if len(b) == 0 {
		return nil
	}
	return string(b)
}

// ErrNotFound helpers
func IsNotFound(err error) bool {
	return err == sql.ErrNoRows
}

func NotFoundf(format string, args ...any) error {
	return fmt.Errorf("%w: %s", sql.ErrNoRows, fmt.Sprintf(format, args...))
}
