package pipeline_test

import (
	"context"
	"encoding/json"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/ccc112233/dag/internal/pipeline"
	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/internal/store"
	"github.com/ccc112233/dag/internal/tscache"
	northstub "github.com/ccc112233/dag/plugins/north/stub"
	southstub "github.com/ccc112233/dag/plugins/south/stub"
)

func TestStubSouthToStubNorthPipeline(t *testing.T) {
	northstub.Reset()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()

	reg := plugin.NewRegistry()
	reg.RegisterSouth(southstub.Driver{})
	reg.RegisterNorth(northstub.Driver{})
	eng := pipeline.New(st, reg, slog.New(slog.NewTextHandler(os.Stderr, nil)), 100)
	defer eng.Close()

	ctx := context.Background()
	connID, pointID, taskID := "c1", "p1", "t1"
	_ = st.UpsertSouthConnection(ctx, store.SouthConnection{
		ID: connID, Name: "stub", DriverType: "stub_south",
		EndpointJSON: json.RawMessage(`{"startValue":10,"step":1}`), TimeoutMs: 2000,
	})
	_ = st.UpsertPoint(ctx, store.Point{
		ID: pointID, ConnectionID: connID, Name: "temp",
		AddressJSON: json.RawMessage(`{"n":1}`), DataType: "float",
	})
	ps, _ := json.Marshal([]string{pointID})
	_ = st.UpsertCollectTask(ctx, store.CollectTask{
		ID: taskID, ConnectionID: connID, Mode: "poll", IntervalMs: 100, PointSetJSON: ps,
	})
	_ = st.UpsertThingModel(ctx, store.ThingModel{
		ID: "mrow", ModelID: "motor", Version: "1.0.0", Name: "Motor",
		Properties: []store.PropertyDef{
			{ID: "temp", Name: "Temperature", DataType: "float", Unit: "C"},
			{ID: "rpm", Name: "RPM", DataType: "float"},
			{ID: "on", Name: "On", DataType: "bool"},
		},
	})
	_ = st.UpsertDevice(ctx, store.Device{ID: "d1", DeviceID: "dev-1", ModelID: "motor", ModelVersion: "1.0.0", Name: "M1"})
	_ = st.UpsertMapping(ctx, store.Mapping{ID: "map1", DeviceID: "d1", PointID: pointID, PropertyID: "temp", Scale: 0.1, Offset: 1})
	_ = st.UpsertNorthChannel(ctx, store.NorthChannel{
		ID: "n1", Name: "stub", ChannelType: "stub_north",
		ConfigJSON: json.RawMessage(`{}`), Enabled: true, Trigger: "change",
	})
	_ = st.BindChannelDevice(ctx, "n1", "d1")

	if err := eng.StartTask(ctx, taskID); err != nil {
		t.Fatal(err)
	}

	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		snaps := northstub.Snapshots()
		if len(snaps) > 0 {
			env := snaps[0]
			if env.DeviceID != "dev-1" || env.ModelID != "motor" {
				t.Fatalf("unexpected envelope: %+v", env)
			}
			pv, ok := env.Properties["temp"]
			if !ok {
				t.Fatalf("missing temp: %+v", env.Properties)
			}
			if pv.Quality != plugin.QualityGood {
				t.Fatalf("quality=%s", pv.Quality)
			}
			// value = 10*0.1 + 1 = 2
			if f, ok := pv.Value.(float64); !ok || f != 2 {
				t.Fatalf("scaled value want 2 got %#v", pv.Value)
			}
			m, _ := st.GetMetric(ctx, "task", taskID)
			if m.SuccessCount < 1 {
				t.Fatalf("expected success metric")
			}
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	m, _ := st.GetMetric(ctx, "task", taskID)
	t.Fatalf("timeout waiting for stub_north envelope; taskMetric=%+v snaps=%d", m, len(northstub.Snapshots()))
}

func TestIsolationFailDoesNotStopGood(t *testing.T) {
	northstub.Reset()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := plugin.NewRegistry()
	reg.RegisterSouth(southstub.Driver{})
	reg.RegisterSouth(southstub.FailDriver{})
	reg.RegisterNorth(northstub.Driver{})
	eng := pipeline.New(st, reg, slog.Default(), 100)
	defer eng.Close()
	ctx := context.Background()

	_ = st.UpsertSouthConnection(ctx, store.SouthConnection{ID: "good", Name: "g", DriverType: "stub_south", EndpointJSON: json.RawMessage(`{}`), TimeoutMs: 1000})
	_ = st.UpsertSouthConnection(ctx, store.SouthConnection{ID: "bad", Name: "b", DriverType: "stub_south_fail", EndpointJSON: json.RawMessage(`{}`), TimeoutMs: 1000})
	_ = st.UpsertPoint(ctx, store.Point{ID: "pg", ConnectionID: "good", AddressJSON: json.RawMessage(`{}`)})
	_ = st.UpsertPoint(ctx, store.Point{ID: "pb", ConnectionID: "bad", AddressJSON: json.RawMessage(`{}`)})
	psg, _ := json.Marshal([]string{"pg"})
	psb, _ := json.Marshal([]string{"pb"})
	_ = st.UpsertCollectTask(ctx, store.CollectTask{ID: "tg", ConnectionID: "good", Mode: "poll", IntervalMs: 100, PointSetJSON: psg})
	_ = st.UpsertCollectTask(ctx, store.CollectTask{ID: "tb", ConnectionID: "bad", Mode: "poll", IntervalMs: 100, PointSetJSON: psb})
	_ = st.UpsertThingModel(ctx, store.ThingModel{ID: "m", ModelID: "x", Version: "1", Name: "x", Properties: []store.PropertyDef{{ID: "a", Name: "a", DataType: "float"}}})
	_ = st.UpsertDevice(ctx, store.Device{ID: "d", DeviceID: "dx", ModelID: "x", ModelVersion: "1", Name: "d"})
	_ = st.UpsertMapping(ctx, store.Mapping{ID: "mp", DeviceID: "d", PointID: "pg", PropertyID: "a", Scale: 1})
	_ = st.UpsertNorthChannel(ctx, store.NorthChannel{ID: "n", Name: "n", ChannelType: "stub_north", ConfigJSON: json.RawMessage(`{}`), Enabled: true, Trigger: "change"})
	_ = st.BindChannelDevice(ctx, "n", "d")

	if err := eng.StartTask(ctx, "tg"); err != nil {
		t.Fatal(err)
	}
	_ = eng.StartTask(ctx, "tb") // open ok; reads fail and must not block good task

	deadline := time.Now().Add(3 * time.Second)
	var mg, mb *store.RuntimeMetric
	for time.Now().Before(deadline) {
		mg, _ = st.GetMetric(ctx, "task", "tg")
		mb, _ = st.GetMetric(ctx, "task", "tb")
		if mg.SuccessCount >= 2 && mb.FailCount >= 1 && mb.LastError != "" && len(northstub.Snapshots()) >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("AC-S3 isolation failed: good=%+v bad=%+v snaps=%d", mg, mb, len(northstub.Snapshots()))
}

func TestBufferReplay(t *testing.T) {
	northstub.Reset()
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	reg := plugin.NewRegistry()
	reg.RegisterSouth(southstub.Driver{})
	reg.RegisterNorth(northstub.Driver{})
	eng := pipeline.New(st, reg, slog.Default(), 100)
	defer eng.Close()
	ctx := context.Background()

	_ = st.UpsertSouthConnection(ctx, store.SouthConnection{ID: "c", Name: "c", DriverType: "stub_south", EndpointJSON: json.RawMessage(`{}`), TimeoutMs: 1000})
	_ = st.UpsertPoint(ctx, store.Point{ID: "p", ConnectionID: "c", AddressJSON: json.RawMessage(`{}`)})
	ps, _ := json.Marshal([]string{"p"})
	_ = st.UpsertCollectTask(ctx, store.CollectTask{ID: "t", ConnectionID: "c", Mode: "poll", IntervalMs: 80, PointSetJSON: ps})
	_ = st.UpsertThingModel(ctx, store.ThingModel{ID: "m", ModelID: "x", Version: "1", Name: "x", Properties: []store.PropertyDef{{ID: "a", Name: "a", DataType: "float"}}})
	_ = st.UpsertDevice(ctx, store.Device{ID: "d", DeviceID: "dx", ModelID: "x", ModelVersion: "1", Name: "d"})
	_ = st.UpsertMapping(ctx, store.Mapping{ID: "mp", DeviceID: "d", PointID: "p", PropertyID: "a", Scale: 1})
	_ = st.UpsertNorthChannel(ctx, store.NorthChannel{
		ID: "n", Name: "n", ChannelType: "stub_north",
		ConfigJSON: json.RawMessage(`{"failTimes":100}`), Enabled: true, Trigger: "change",
	})
	_ = st.BindChannelDevice(ctx, "n", "d")

	if err := eng.StartTask(ctx, "t"); err != nil {
		t.Fatal(err)
	}

	// Phase 1: forced publish failures must enqueue outbound buffer.
	deadline := time.Now().Add(3 * time.Second)
	buffered := false
	for time.Now().Before(deadline) {
		n, _ := st.BufferCount(ctx, "n")
		if n >= 1 {
			buffered = true
			break
		}
		time.Sleep(40 * time.Millisecond)
	}
	if !buffered {
		t.Fatalf("expected BufferCount>=1 while north failing; snaps=%d", len(northstub.Snapshots()))
	}

	// Phase 2: stop sampling; clear forced failures; active flush must drain buffer.
	_ = eng.StopTask(ctx, "t")
	northstub.Allow()
	eng.InvalidatePublisher("n")
	deadline = time.Now().Add(4 * time.Second)
	for time.Now().Before(deadline) {
		eng.FlushBufferedChannels(ctx)
		n, _ := st.BufferCount(ctx, "n")
		snaps := northstub.Snapshots()
		if n == 0 && len(snaps) >= 1 {
			return
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("expected proactive replay: buffer=%d snaps=%d", mustCount(st, "n"), len(northstub.Snapshots()))
}

func mustCount(st *store.Store, id string) int {
	n, _ := st.BufferCount(context.Background(), id)
	return n
}

// TestPipelineTSCache40kPerSec drives stub south → engine.handleSamples → tscache
// at ≥40000 persisted samples/s with zero drops (single-service write path).
func TestPipelineTSCache40kPerSec(t *testing.T) {
	dir := t.TempDir()
	st, err := store.Open(filepath.Join(dir, "t.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	ts, err := tscache.Open(tscache.Config{Path: filepath.Join(dir, "ts"), RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer ts.Close()

	reg := plugin.NewRegistry()
	reg.RegisterSouth(southstub.Driver{})
	eng := pipeline.New(st, reg, slog.Default(), 100)
	eng.SetTSCache(ts)
	defer eng.Close()
	ctx := context.Background()

	// 4000 pts × 50ms poll ≈ 80k/s theoretical; ≥1s run must clear 40k/s after start overhead.
	const points = 4000
	const intervalMs = 50
	_ = st.UpsertSouthConnection(ctx, store.SouthConnection{
		ID: "c-perf", Name: "perf", DriverType: "stub_south",
		EndpointJSON: json.RawMessage(`{"startValue":0,"step":1}`), TimeoutMs: 2000,
	})
	ids := make([]string, 0, points)
	for i := 0; i < points; i++ {
		pid := "pp-" + strconv.Itoa(i)
		ids = append(ids, pid)
		if err := st.UpsertPoint(ctx, store.Point{
			ID: pid, ConnectionID: "c-perf", Name: pid,
			AddressJSON: json.RawMessage(`{}`), DataType: "float",
		}); err != nil {
			t.Fatal(err)
		}
	}
	ps, _ := json.Marshal(ids)
	_ = st.UpsertCollectTask(ctx, store.CollectTask{
		ID: "t-perf", ConnectionID: "c-perf", Mode: "poll",
		IntervalMs: intervalMs, PointSetJSON: ps,
	})

	start := time.Now()
	if err := eng.StartTask(ctx, "t-perf"); err != nil {
		t.Fatal(err)
	}
	time.Sleep(1100 * time.Millisecond)
	_ = eng.StopTask(ctx, "t-perf")
	if err := ts.Flush(); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)

	written, dropped := ts.Stats()
	if dropped != 0 {
		t.Fatalf("pipeline tscache dropped=%d written=%d", dropped, written)
	}
	rate := float64(written) / elapsed.Seconds()
	t.Logf("pipeline persisted %d samples in %s => %.0f pts/s", written, elapsed, rate)
	if written < 40_000 {
		t.Fatalf("written=%d < 40000 required for ≥1s at 40k/s", written)
	}
	if rate < 40_000 {
		t.Fatalf("pipeline persist rate %.0f pts/s < 40000", rate)
	}
}
