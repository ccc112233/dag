package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sync"
	"time"

	"github.com/ccc112233/dag/internal/plugin"
	"github.com/ccc112233/dag/internal/store"
	"github.com/ccc112233/dag/internal/tscache"
	"github.com/google/uuid"
)

// Engine schedules collect tasks and routes mapped envelopes north.
type Engine struct {
	store    *store.Store
	registry *plugin.Registry
	log      *slog.Logger
	maxBuf   int
	ts       *tscache.Cache

	mu       sync.Mutex
	running  map[string]context.CancelFunc
	pubs     map[string]plugin.Publisher
	lastProp map[string]map[string]plugin.PropertyValue // deviceID -> prop -> last
	seq      map[string]int64
	lastInt  map[string]time.Time // channelID -> last interval send

	flushStop chan struct{}
	flushWG   sync.WaitGroup
}

// New creates an engine.
func New(st *store.Store, reg *plugin.Registry, log *slog.Logger, maxBuf int) *Engine {
	if maxBuf <= 0 {
		maxBuf = 10000
	}
	if log == nil {
		log = slog.Default()
	}
	e := &Engine{
		store:     st,
		registry:  reg,
		log:       log,
		maxBuf:    maxBuf,
		running:   map[string]context.CancelFunc{},
		pubs:      map[string]plugin.Publisher{},
		lastProp:  map[string]map[string]plugin.PropertyValue{},
		seq:       map[string]int64{},
		lastInt:   map[string]time.Time{},
		flushStop: make(chan struct{}),
	}
	e.flushWG.Add(1)
	go e.flushLoop()
	return e
}

// SetTSCache attaches the local time-series sample cache (7-day retention by default).
func (e *Engine) SetTSCache(c *tscache.Cache) {
	e.ts = c
}

// TSCache returns the attached local TS cache, if any.
func (e *Engine) TSCache() *tscache.Cache { return e.ts }

// StartTask starts a collect task goroutine.
func (e *Engine) StartTask(ctx context.Context, taskID string) error {
	e.mu.Lock()
	if _, ok := e.running[taskID]; ok {
		e.mu.Unlock()
		return nil
	}
	e.mu.Unlock()

	task, err := e.store.GetCollectTask(ctx, taskID)
	if err != nil {
		return err
	}
	conn, err := e.store.GetSouthConnection(ctx, task.ConnectionID)
	if err != nil {
		return err
	}
	driver, err := e.registry.South(conn.DriverType)
	if err != nil {
		return err
	}
	if err := driver.ValidateConfig(conn.EndpointJSON); err != nil {
		return err
	}
	creds := plugin.ResolveCredentials(conn.CredentialsRef)
	openCtx, cancelOpen := context.WithTimeout(ctx, time.Duration(conn.TimeoutMs)*time.Millisecond)
	defer cancelOpen()
	sess, err := driver.Open(openCtx, conn.EndpointJSON, creds)
	if err != nil {
		_ = e.store.IncrMetricFail(ctx, "task", taskID, err.Error())
		return err
	}

	var pointIDs []string
	_ = json.Unmarshal(task.PointSetJSON, &pointIDs)
	points := make([]plugin.PointAddr, 0, len(pointIDs))
	for _, pid := range pointIDs {
		p, err := e.store.GetPoint(ctx, pid)
		if err != nil {
			_ = sess.Close()
			return err
		}
		points = append(points, plugin.PointAddr{
			ID: p.ID, Name: p.Name, Address: p.AddressJSON, DataType: p.DataType,
		})
	}

	runCtx, cancel := context.WithCancel(context.Background())
	e.mu.Lock()
	e.running[taskID] = cancel
	e.mu.Unlock()
	_ = e.store.SetTaskEnabled(ctx, taskID, true)

	go e.loopTask(runCtx, task, sess, points)
	return nil
}

// StopTask stops a running task.
func (e *Engine) StopTask(ctx context.Context, taskID string) error {
	e.mu.Lock()
	cancel, ok := e.running[taskID]
	if ok {
		delete(e.running, taskID)
	}
	e.mu.Unlock()
	if ok {
		cancel()
	}
	return e.store.SetTaskEnabled(ctx, taskID, false)
}

func (e *Engine) loopTask(ctx context.Context, task *store.CollectTask, sess plugin.Session, points []plugin.PointAddr) {
	defer func() {
		if r := recover(); r != nil {
			e.log.Error("task panic recovered", "taskId", task.ID, "panic", r)
			_ = e.store.IncrMetricFail(context.Background(), "task", task.ID, fmt.Sprintf("panic: %v", r))
		}
		_ = sess.Close()
		e.mu.Lock()
		delete(e.running, task.ID)
		e.mu.Unlock()
	}()

	interval := time.Duration(task.IntervalMs) * time.Millisecond
	if interval <= 0 {
		interval = time.Second
	}

	if task.Mode == "subscribe" {
		sink := make(chan plugin.Sample, 64)
		sub, err := sess.Subscribe(ctx, points, sink)
		if err != nil {
			_ = e.store.IncrMetricFail(context.Background(), "task", task.ID, err.Error())
			e.pollLoop(ctx, task, sess, points, interval)
			return
		}
		defer func() { _ = sub.Unsubscribe() }()
		batch := map[string]plugin.Sample{}
		flush := time.NewTicker(interval)
		defer flush.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case sm, ok := <-sink:
				if !ok {
					return
				}
				batch[sm.PointID] = sm
			case <-flush.C:
				if len(batch) == 0 {
					continue
				}
				samples := make([]plugin.Sample, 0, len(batch))
				for _, sm := range batch {
					samples = append(samples, sm)
				}
				batch = map[string]plugin.Sample{}
				e.handleSamples(ctx, task.ID, samples)
			}
		}
	}
	e.pollLoop(ctx, task, sess, points, interval)
}

func (e *Engine) pollLoop(ctx context.Context, task *store.CollectTask, sess plugin.Session, points []plugin.PointAddr, interval time.Duration) {
	t := time.NewTicker(interval)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-t.C:
			func() {
				defer func() {
					if r := recover(); r != nil {
						e.log.Error("read panic", "taskId", task.ID, "panic", r)
						_ = e.store.IncrMetricFail(context.Background(), "task", task.ID, fmt.Sprintf("panic: %v", r))
					}
				}()
				rctx, cancel := context.WithTimeout(ctx, interval)
				defer cancel()
				samples, err := sess.Read(rctx, points)
				if err != nil {
					_ = e.store.IncrMetricFail(context.Background(), "task", task.ID, err.Error())
					return
				}
				e.handleSamples(ctx, task.ID, samples)
			}()
		}
	}
}

func (e *Engine) handleSamples(ctx context.Context, taskID string, samples []plugin.Sample) {
	now := time.Now().UTC()
	_ = e.store.IncrMetricSuccess(context.Background(), "task", taskID, now)

	if e.ts != nil {
		for _, sm := range samples {
			ts := sm.Timestamp
			if ts.IsZero() {
				ts = now
			}
			q := uint8(0)
			switch sm.Quality {
			case plugin.QualityGood:
				q = 1
			case plugin.QualityUncertain:
				q = 2
			default:
				q = 0
			}
			e.ts.WriteNumeric(sm.PointID, ts, sm.Value, q)
		}
	}

	// Find devices whose mappings reference these points.
	mappings, err := e.store.ListMappings(ctx)
	if err != nil {
		return
	}
	byDevice := map[string][]store.Mapping{}
	pointSet := map[string]plugin.Sample{}
	for _, sm := range samples {
		pointSet[sm.PointID] = sm
	}
	for _, m := range mappings {
		if _, ok := pointSet[m.PointID]; ok {
			byDevice[m.DeviceID] = append(byDevice[m.DeviceID], m)
		}
	}
	for deviceKey, maps := range byDevice {
		dev, err := e.store.GetDevice(ctx, deviceKey)
		if err != nil {
			// try by business device_id
			dev, err = e.store.GetDeviceByDeviceID(ctx, deviceKey)
			if err != nil {
				continue
			}
		}
		props := map[string]plugin.PropertyValue{}
		var ts time.Time
		var sources []string
		for _, m := range maps {
			sm := pointSet[m.PointID]
			val := applyScale(sm.Value, m.Scale, m.Offset)
			props[m.PropertyID] = plugin.PropertyValue{Value: val, Quality: sm.Quality}
			if sm.Timestamp.After(ts) {
				ts = sm.Timestamp
			}
			if sm.Address != "" {
				sources = append(sources, sm.Address)
			}
		}
		if ts.IsZero() {
			ts = now
		}
		env := plugin.Envelope{
			MessageID:    uuid.NewString(),
			DeviceID:     dev.DeviceID,
			ModelID:      dev.ModelID,
			ModelVersion: dev.ModelVersion,
			Timestamp:    ts,
			Properties:   props,
			Source:       join(sources),
		}
		e.route(ctx, dev, env)
	}
}

func applyScale(v any, scale, offset float64) any {
	if scale == 0 {
		scale = 1
	}
	switch x := v.(type) {
	case float64:
		return x*scale + offset
	case float32:
		return float64(x)*scale + offset
	case int:
		return float64(x)*scale + offset
	case int16:
		return float64(x)*scale + offset
	case int32:
		return float64(x)*scale + offset
	case int64:
		return float64(x)*scale + offset
	case uint16:
		return float64(x)*scale + offset
	case uint32:
		return float64(x)*scale + offset
	case json.Number:
		f, err := x.Float64()
		if err == nil {
			return f*scale + offset
		}
	}
	return v
}

func join(ss []string) string {
	if len(ss) == 0 {
		return ""
	}
	b, _ := json.Marshal(ss)
	return string(b)
}

func (e *Engine) route(ctx context.Context, dev *store.Device, env plugin.Envelope) {
	channels, err := e.store.ListChannelsForDevice(ctx, dev.ID)
	if err != nil || len(channels) == 0 {
		// also try binding by business device id
		channels, err = e.store.ListChannelsForDevice(ctx, dev.DeviceID)
		if err != nil {
			return
		}
	}
	for _, ch := range channels {
		send := false
		switch ch.Trigger {
		case "interval":
			send = e.intervalDue(ch)
		default: // change
			send = e.changed(dev.DeviceID, env.Properties)
		}
		if !send {
			continue
		}
		if err := e.publishOrBuffer(ctx, ch, env); err != nil {
			e.log.Warn("north publish failed", "channel", ch.ID, "err", err)
		}
	}
}

func (e *Engine) intervalDue(ch store.NorthChannel) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	iv := time.Duration(ch.IntervalMs) * time.Millisecond
	if iv <= 0 {
		iv = time.Second
	}
	last, ok := e.lastInt[ch.ID]
	now := time.Now()
	if !ok || now.Sub(last) >= iv {
		e.lastInt[ch.ID] = now
		return true
	}
	return false
}

func (e *Engine) changed(deviceID string, props map[string]plugin.PropertyValue) bool {
	e.mu.Lock()
	defer e.mu.Unlock()
	prev, ok := e.lastProp[deviceID]
	if !ok {
		e.lastProp[deviceID] = cloneProps(props)
		return true
	}
	changed := false
	for k, v := range props {
		pv, ok := prev[k]
		if !ok || fmt.Sprint(pv.Value) != fmt.Sprint(v.Value) || pv.Quality != v.Quality {
			changed = true
			break
		}
	}
	e.lastProp[deviceID] = cloneProps(props)
	return changed
}

func cloneProps(in map[string]plugin.PropertyValue) map[string]plugin.PropertyValue {
	out := make(map[string]plugin.PropertyValue, len(in))
	for k, v := range in {
		out[k] = v
	}
	return out
}

func (e *Engine) publishOrBuffer(ctx context.Context, ch store.NorthChannel, env plugin.Envelope) error {
	pub, err := e.getPublisher(ctx, ch)
	if err != nil {
		e.InvalidatePublisher(ch.ID)
		_ = e.buffer(ctx, ch.ID, env)
		_ = e.store.IncrMetricFail(ctx, "channel", ch.ID, err.Error())
		return err
	}
	if err := pub.Publish(ctx, env); err != nil {
		e.InvalidatePublisher(ch.ID)
		_ = e.buffer(ctx, ch.ID, env)
		_ = e.store.IncrMetricFail(ctx, "channel", ch.ID, err.Error())
		return err
	}
	_ = e.store.IncrMetricSuccess(ctx, "channel", ch.ID, time.Now().UTC())
	// try flush buffered
	go e.flushChannel(ch.ID)
	return nil
}

func (e *Engine) getPublisher(ctx context.Context, ch store.NorthChannel) (plugin.Publisher, error) {
	e.mu.Lock()
	if p, ok := e.pubs[ch.ID]; ok {
		e.mu.Unlock()
		return p, nil
	}
	e.mu.Unlock()
	driver, err := e.registry.North(ch.ChannelType)
	if err != nil {
		return nil, err
	}
	if err := driver.ValidateConfig(ch.ConfigJSON); err != nil {
		return nil, err
	}
	creds := plugin.ResolveCredentials(ch.CredentialsRef)
	pub, err := driver.Open(ctx, ch.ConfigJSON, creds)
	if err != nil {
		return nil, err
	}
	e.mu.Lock()
	e.pubs[ch.ID] = pub
	e.mu.Unlock()
	return pub, nil
}

// InvalidatePublisher drops cached publisher (e.g. after config change).
func (e *Engine) InvalidatePublisher(channelID string) {
	e.mu.Lock()
	defer e.mu.Unlock()
	if p, ok := e.pubs[channelID]; ok {
		_ = p.Close()
		delete(e.pubs, channelID)
	}
}

func (e *Engine) buffer(ctx context.Context, channelID string, env plugin.Envelope) error {
	n, _ := e.store.BufferCount(ctx, channelID)
	for n >= e.maxBuf {
		e.log.Warn("outbound buffer full, dropping oldest", "channel", channelID)
		_ = e.store.DropOldestBuffer(ctx, channelID)
		n--
	}
	e.mu.Lock()
	seq := e.seq[channelID]
	if seq == 0 {
		ns, err := e.store.NextBufferSeq(ctx, channelID)
		if err == nil {
			seq = ns - 1
		}
	}
	seq++
	e.seq[channelID] = seq
	e.mu.Unlock()
	b, err := json.Marshal(env)
	if err != nil {
		return err
	}
	return e.store.EnqueueBuffer(ctx, channelID, seq, b)
}

func (e *Engine) flushChannel(channelID string) {
	ctx := context.Background()
	ch, err := e.store.GetNorthChannel(ctx, channelID)
	if err != nil || !ch.Enabled {
		return
	}
	pub, err := e.getPublisher(ctx, *ch)
	if err != nil {
		e.InvalidatePublisher(channelID)
		return
	}
	items, err := e.store.ListBuffer(ctx, channelID, 100)
	if err != nil {
		return
	}
	for _, it := range items {
		var env plugin.Envelope
		if err := json.Unmarshal(it.Payload, &env); err != nil {
			_ = e.store.DeleteBuffer(ctx, it.ID)
			continue
		}
		if err := pub.Publish(ctx, env); err != nil {
			e.InvalidatePublisher(channelID)
			_ = e.store.IncrMetricFail(ctx, "channel", channelID, err.Error())
			return
		}
		_ = e.store.DeleteBuffer(ctx, it.ID)
		_ = e.store.IncrMetricSuccess(ctx, "channel", channelID, time.Now().UTC())
	}
}

func (e *Engine) flushLoop() {
	defer e.flushWG.Done()
	t := time.NewTicker(500 * time.Millisecond)
	defer t.Stop()
	for {
		select {
		case <-e.flushStop:
			return
		case <-t.C:
			e.FlushBufferedChannels(context.Background())
		}
	}
}

// FlushBufferedChannels actively replays outbound buffers for enabled channels (N-04).
func (e *Engine) FlushBufferedChannels(ctx context.Context) {
	channels, err := e.store.ListNorthChannels(ctx)
	if err != nil {
		return
	}
	for _, ch := range channels {
		if !ch.Enabled {
			continue
		}
		n, err := e.store.BufferCount(ctx, ch.ID)
		if err != nil || n == 0 {
			continue
		}
		e.flushChannel(ch.ID)
	}
}

// RestoreEnabledTasks starts tasks marked enabled in DB (process restart).
// Tasks are restored in parallel with per-task Open timeouts so one slow Open cannot block others (S-05).
func (e *Engine) RestoreEnabledTasks(ctx context.Context) {
	tasks, err := e.store.ListCollectTasks(ctx)
	if err != nil {
		return
	}
	var wg sync.WaitGroup
	for _, t := range tasks {
		if !t.Enabled {
			continue
		}
		wg.Add(1)
		go func(taskID string) {
			defer wg.Done()
			connTimeout := 5 * time.Second
			if task, err := e.store.GetCollectTask(ctx, taskID); err == nil {
				if conn, err := e.store.GetSouthConnection(ctx, task.ConnectionID); err == nil && conn.TimeoutMs > 0 {
					connTimeout = time.Duration(conn.TimeoutMs) * time.Millisecond
				}
			}
			tctx, cancel := context.WithTimeout(ctx, connTimeout+time.Second)
			defer cancel()
			if err := e.StartTask(tctx, taskID); err != nil {
				e.log.Warn("restore task failed", "taskId", taskID, "err", err)
			}
		}(t.ID)
	}
	wg.Wait()
}

// Close stops all tasks and publishers.
func (e *Engine) Close() {
	select {
	case <-e.flushStop:
	default:
		close(e.flushStop)
	}
	e.flushWG.Wait()

	e.mu.Lock()
	defer e.mu.Unlock()
	for id, cancel := range e.running {
		cancel()
		delete(e.running, id)
	}
	for id, p := range e.pubs {
		_ = p.Close()
		delete(e.pubs, id)
	}
}
