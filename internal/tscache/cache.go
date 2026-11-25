// Package tscache is a lightweight append-only local time-series cache for
// edge sample retention. Configuration/metadata stay in SQLite; high-rate
// point samples land here for 7-day (configurable) local caching.
package tscache

import (
	"bufio"
	"encoding/binary"
	"encoding/json"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"math"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"sync/atomic"
	"time"
)

const (
	magic          = "DAPTS01\n"
	defaultRetDays = 7
	flushEvery     = 4096
	flushInterval  = 5 * time.Millisecond
	chanBuf        = 262144
	// ApproxBytesPerSample is the typical on-disk size used for capacity planning
	// (hash+len+short-id+timestamp+value+quality). Real size grows with PointID length.
	ApproxBytesPerSample = 40
)

// Sample is one cached point reading.
type Sample struct {
	PointID   string
	Timestamp time.Time
	Value     float64
	Quality   uint8
}

// Config controls the cache.
type Config struct {
	Path          string
	RetentionDays int
}

type flushReq struct {
	done chan error
}

// Cache is an embedded day-partitioned TS store with async batched writes.
type Cache struct {
	dir       string
	retention time.Duration

	in     chan Sample
	flushQ chan flushReq
	stop   chan struct{}
	wg     sync.WaitGroup
	closed atomic.Bool

	written atomic.Uint64
	dropped atomic.Uint64

	mu      sync.Mutex
	curDay  string
	curFile *os.File
	curBuf  *bufio.Writer
}

// Open creates or opens a cache under dir.
func Open(cfg Config) (*Cache, error) {
	if cfg.Path == "" {
		return nil, errors.New("tscache: path required")
	}
	days := cfg.RetentionDays
	if days <= 0 {
		days = defaultRetDays
	}
	if err := os.MkdirAll(cfg.Path, 0o755); err != nil {
		return nil, err
	}
	c := &Cache{
		dir:       cfg.Path,
		retention: time.Duration(days) * 24 * time.Hour,
		in:        make(chan Sample, chanBuf),
		flushQ:    make(chan flushReq, 8),
		stop:      make(chan struct{}),
	}
	c.wg.Add(1)
	go c.writerLoop()
	c.wg.Add(1)
	go c.purgeLoop()
	return c, nil
}

// Write enqueues samples, blocking until accepted or the cache is closed.
// It does not silently drop new samples under backpressure.
func (c *Cache) Write(samples ...Sample) {
	if c == nil || c.closed.Load() {
		return
	}
	for _, s := range samples {
		if s.PointID == "" {
			continue
		}
		if s.Timestamp.IsZero() {
			s.Timestamp = time.Now().UTC()
		}
		select {
		case c.in <- s:
		case <-c.stop:
			c.dropped.Add(1)
			return
		}
	}
}

// WriteNumeric converts common numeric values and writes.
func (c *Cache) WriteNumeric(pointID string, ts time.Time, v any, quality uint8) {
	f, ok := toFloat(v)
	if !ok {
		return
	}
	c.Write(Sample{PointID: pointID, Timestamp: ts, Value: f, Quality: quality})
}

// Stats returns write counters.
func (c *Cache) Stats() (written, dropped uint64) {
	return c.written.Load(), c.dropped.Load()
}

// Flush drains the inbound queue, persists the current batch, and fsyncs the
// active day file so subsequent Query/reopen can observe the data.
func (c *Cache) Flush() error {
	if c == nil || c.closed.Load() {
		return nil
	}
	req := flushReq{done: make(chan error, 1)}
	select {
	case c.flushQ <- req:
	case <-c.stop:
		return nil
	}
	select {
	case err := <-req.done:
		return err
	case <-time.After(30 * time.Second):
		return errors.New("tscache: flush timeout")
	}
}

// Close stops writers, flushes, syncs, and closes files.
func (c *Cache) Close() error {
	if c == nil || !c.closed.CompareAndSwap(false, true) {
		return nil
	}
	close(c.stop)
	c.wg.Wait()
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.closeFileLocked(true)
}

// Query returns samples for pointID in [from, to] (inclusive), newest last.
func (c *Cache) Query(pointID string, from, to time.Time, limit int) ([]Sample, error) {
	if limit <= 0 {
		limit = 10000
	}
	if to.IsZero() {
		to = time.Now().UTC()
	}
	if from.IsZero() {
		from = to.Add(-c.retention)
	}
	want := hashID(pointID)
	days := dayRange(from, to)
	var out []Sample
	for _, day := range days {
		path := filepath.Join(c.dir, day+".pts")
		f, err := os.Open(path)
		if err != nil {
			continue
		}
		recs, err := readDay(f, want, pointID, from, to)
		_ = f.Close()
		if err != nil {
			return out, err
		}
		out = append(out, recs...)
	}
	if len(out) > limit {
		out = out[len(out)-limit:]
	}
	return out, nil
}

// Retention returns configured retention.
func (c *Cache) Retention() time.Duration { return c.retention }

// Dir returns storage directory.
func (c *Cache) Dir() string { return c.dir }

// EstimateBytes estimates on-disk bytes for n samples at the given average id length.
func EstimateBytes(n int64, avgIDLen int) int64 {
	if avgIDLen < 0 {
		avgIDLen = 0
	}
	per := int64(4 + 2 + avgIDLen + 8 + 8 + 1 + 1)
	return n * per
}

func (c *Cache) writerLoop() {
	defer c.wg.Done()
	buf := make([]Sample, 0, flushEvery)
	ticker := time.NewTicker(flushInterval)
	defer ticker.Stop()
	flush := func(syncDisk bool) error {
		if len(buf) > 0 {
			n := len(buf)
			if err := c.appendBatch(buf, syncDisk); err != nil {
				return err
			}
			c.written.Add(uint64(n))
			buf = buf[:0]
			return nil
		}
		if syncDisk {
			c.mu.Lock()
			defer c.mu.Unlock()
			return c.syncFileLocked()
		}
		return nil
	}
	drainInbound := func() {
		for {
			select {
			case s := <-c.in:
				buf = append(buf, s)
			default:
				return
			}
		}
	}
	for {
		select {
		case <-c.stop:
			drainInbound()
			_ = flush(true)
			return
		case req := <-c.flushQ:
			drainInbound()
			req.done <- flush(true)
		case s := <-c.in:
			buf = append(buf, s)
			if len(buf) >= flushEvery {
				_ = flush(false)
			}
		case <-ticker.C:
			_ = flush(false)
		}
	}
}

func (c *Cache) purgeLoop() {
	defer c.wg.Done()
	t := time.NewTicker(time.Hour)
	defer t.Stop()
	c.purgeOnce()
	for {
		select {
		case <-c.stop:
			return
		case <-t.C:
			c.purgeOnce()
		}
	}
}

func (c *Cache) purgeOnce() {
	cutoff := time.Now().UTC().Add(-c.retention).Format("20060102")
	entries, err := os.ReadDir(c.dir)
	if err != nil {
		return
	}
	for _, e := range entries {
		name := e.Name()
		if len(name) < 8 || filepath.Ext(name) != ".pts" {
			continue
		}
		day := name[:8]
		if day < cutoff {
			_ = os.Remove(filepath.Join(c.dir, name))
		}
	}
}

func (c *Cache) appendBatch(batch []Sample, syncDisk bool) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	byDay := map[string][]Sample{}
	for _, s := range batch {
		day := s.Timestamp.UTC().Format("20060102")
		byDay[day] = append(byDay[day], s)
	}
	days := make([]string, 0, len(byDay))
	for d := range byDay {
		days = append(days, d)
	}
	sort.Strings(days)
	for _, day := range days {
		if err := c.ensureDayLocked(day); err != nil {
			return err
		}
		for _, s := range byDay[day] {
			if err := writeRecord(c.curBuf, s); err != nil {
				return err
			}
		}
		if err := c.curBuf.Flush(); err != nil {
			return err
		}
	}
	if syncDisk {
		return c.syncFileLocked()
	}
	return nil
}

func (c *Cache) ensureDayLocked(day string) error {
	if c.curDay == day && c.curFile != nil {
		return nil
	}
	if err := c.closeFileLocked(true); err != nil {
		return err
	}
	path := filepath.Join(c.dir, day+".pts")
	f, err := os.OpenFile(path, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return err
	}
	fi, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return err
	}
	if fi.Size() == 0 {
		if _, err := f.WriteString(magic); err != nil {
			_ = f.Close()
			return err
		}
	}
	c.curDay = day
	c.curFile = f
	c.curBuf = bufio.NewWriterSize(f, 1<<20)
	return nil
}

func (c *Cache) syncFileLocked() error {
	if c.curBuf != nil {
		if err := c.curBuf.Flush(); err != nil {
			return err
		}
	}
	if c.curFile != nil {
		return c.curFile.Sync()
	}
	return nil
}

func (c *Cache) closeFileLocked(syncDisk bool) error {
	var err error
	if c.curBuf != nil {
		err = c.curBuf.Flush()
		c.curBuf = nil
	}
	if c.curFile != nil {
		if syncDisk {
			if e := c.curFile.Sync(); e != nil && err == nil {
				err = e
			}
		}
		if e := c.curFile.Close(); e != nil && err == nil {
			err = e
		}
		c.curFile = nil
	}
	c.curDay = ""
	return err
}

// Record layout:
// u32 pointHash | u16 idLen | id bytes | i64 unixNano | f64 value | u8 quality | u8 pad
func writeRecord(w io.Writer, s Sample) error {
	id := []byte(s.PointID)
	if len(id) > 65535 {
		return fmt.Errorf("point id too long")
	}
	var hdr [4 + 2]byte
	binary.LittleEndian.PutUint32(hdr[0:4], hashID(s.PointID))
	binary.LittleEndian.PutUint16(hdr[4:6], uint16(len(id)))
	if _, err := w.Write(hdr[:]); err != nil {
		return err
	}
	if _, err := w.Write(id); err != nil {
		return err
	}
	var body [8 + 8 + 1 + 1]byte
	binary.LittleEndian.PutUint64(body[0:8], uint64(s.Timestamp.UTC().UnixNano()))
	binary.LittleEndian.PutUint64(body[8:16], math.Float64bits(s.Value))
	body[16] = s.Quality
	body[17] = 0
	_, err := w.Write(body[:])
	return err
}

func readDay(r io.Reader, wantHash uint32, pointID string, from, to time.Time) ([]Sample, error) {
	br := bufio.NewReaderSize(r, 256*1024)
	head := make([]byte, len(magic))
	if _, err := io.ReadFull(br, head); err != nil {
		return nil, err
	}
	if string(head) != magic {
		return nil, errors.New("tscache: bad magic")
	}
	fromN := from.UnixNano()
	toN := to.UnixNano()
	var out []Sample
	hdr := make([]byte, 6)
	body := make([]byte, 18)
	for {
		if _, err := io.ReadFull(br, hdr); err != nil {
			if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
				return out, nil
			}
			return out, err
		}
		h := binary.LittleEndian.Uint32(hdr[0:4])
		idLen := int(binary.LittleEndian.Uint16(hdr[4:6]))
		idBuf := make([]byte, idLen)
		if _, err := io.ReadFull(br, idBuf); err != nil {
			return out, err
		}
		if _, err := io.ReadFull(br, body); err != nil {
			return out, err
		}
		if h != wantHash || string(idBuf) != pointID {
			continue
		}
		ts := int64(binary.LittleEndian.Uint64(body[0:8]))
		if ts < fromN || ts > toN {
			continue
		}
		out = append(out, Sample{
			PointID:   pointID,
			Timestamp: time.Unix(0, ts).UTC(),
			Value:     math.Float64frombits(binary.LittleEndian.Uint64(body[8:16])),
			Quality:   body[16],
		})
	}
}

func hashID(id string) uint32 {
	h := fnv.New32a()
	_, _ = h.Write([]byte(id))
	return h.Sum32()
}

func dayRange(from, to time.Time) []string {
	f := time.Date(from.UTC().Year(), from.UTC().Month(), from.UTC().Day(), 0, 0, 0, 0, time.UTC)
	t := time.Date(to.UTC().Year(), to.UTC().Month(), to.UTC().Day(), 0, 0, 0, 0, time.UTC)
	var days []string
	for d := f; !d.After(t); d = d.Add(24 * time.Hour) {
		days = append(days, d.Format("20060102"))
	}
	return days
}

func toFloat(v any) (float64, bool) {
	switch x := v.(type) {
	case float64:
		return x, true
	case float32:
		return float64(x), true
	case int:
		return float64(x), true
	case int16:
		return float64(x), true
	case int32:
		return float64(x), true
	case int64:
		return float64(x), true
	case uint16:
		return float64(x), true
	case uint32:
		return float64(x), true
	case uint64:
		return float64(x), true
	case json.Number:
		f, err := x.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}
