package tscache

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func TestWriteQueryAndRetentionConfig(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	now := time.Now().UTC()
	c.Write(
		Sample{PointID: "p1", Timestamp: now.Add(-time.Second), Value: 1.5, Quality: 1},
		Sample{PointID: "p1", Timestamp: now, Value: 2.5, Quality: 1},
		Sample{PointID: "p2", Timestamp: now, Value: 9, Quality: 1},
	)
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}

	got, err := c.Query("p1", now.Add(-time.Minute), now.Add(time.Second), 100)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 {
		t.Fatalf("want 2 samples, got %d", len(got))
	}
	if got[0].Value != 1.5 || got[1].Value != 2.5 {
		t.Fatalf("unexpected values: %+v", got)
	}
	if c.Retention() != 7*24*time.Hour {
		t.Fatalf("retention=%v", c.Retention())
	}
}

func TestWriteThroughput40kPerSec(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "ts")
	c, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}

	const targetRate = 40_000
	const duration = 1200 * time.Millisecond
	now := time.Now().UTC()
	start := time.Now()
	var n int
	for time.Since(start) < duration {
		c.Write(Sample{
			PointID:   "pt-" + itoa(n%40000),
			Timestamp: now.Add(time.Duration(n) * time.Microsecond),
			Value:     float64(n),
			Quality:   1,
		})
		n++
	}
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	elapsed := time.Since(start)
	written, dropped := c.Stats()
	if dropped != 0 {
		t.Fatalf("expected zero drops, got dropped=%d written=%d enqueued=%d", dropped, written, n)
	}
	if written != uint64(n) {
		t.Fatalf("written=%d != enqueued=%d", written, n)
	}
	rate := float64(written) / elapsed.Seconds()
	t.Logf("persisted %d samples in %s => %.0f pts/s (dropped=%d)", written, elapsed, rate, dropped)
	if rate < targetRate {
		t.Fatalf("persisted throughput %.0f pts/s < required %d pts/s", rate, targetRate)
	}

	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	// Close → reopen → Query must see durable samples for a sampled point.
	c2, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	probe := "pt-0"
	got, err := c2.Query(probe, now.Add(-time.Minute), now.Add(time.Hour), 1_000_000)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) == 0 {
		t.Fatalf("reopen query returned 0 samples for %s", probe)
	}
	expectMin := n / 40000
	if expectMin < 1 {
		expectMin = 1
	}
	if len(got) < expectMin {
		t.Fatalf("reopen query got %d samples for %s, want >= %d", len(got), probe, expectMin)
	}
}

func TestFlushSyncDurableAcrossReopen(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	const n = 5000
	for i := 0; i < n; i++ {
		c.Write(Sample{PointID: "dur", Timestamp: now.Add(time.Duration(i) * time.Millisecond), Value: float64(i), Quality: 1})
	}
	if err := c.Flush(); err != nil {
		t.Fatal(err)
	}
	if err := c.Close(); err != nil {
		t.Fatal(err)
	}

	c2, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer c2.Close()
	got, err := c2.Query("dur", now.Add(-time.Second), now.Add(time.Hour), n+10)
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != n {
		t.Fatalf("durable count want %d got %d", n, len(got))
	}
}

func TestPurgeOlderThanRetention(t *testing.T) {
	dir := t.TempDir()
	c, err := Open(Config{Path: dir, RetentionDays: 7})
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()

	oldDay := time.Now().UTC().Add(-10 * 24 * time.Hour).Format("20060102")
	oldPath := filepath.Join(dir, oldDay+".pts")
	if err := os.WriteFile(oldPath, []byte("DAPTS01\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	c.purgeOnce()
	if _, err := os.Stat(oldPath); !os.IsNotExist(err) {
		t.Fatalf("expected old day file purged, err=%v", err)
	}
}

func TestSevenDayCapacityEstimate(t *testing.T) {
	// 40k pts/s × 7d with short ids (~8 bytes) → planning estimate for ops.
	const rate = 40_000
	const days = 7
	n := int64(rate) * 24 * 3600 * days
	bytes := EstimateBytes(n, 8)
	gb := float64(bytes) / (1024 * 1024 * 1024)
	t.Logf("7d@%dpts/s ≈ %.1f GiB (avgIDLen=8, samples=%d)", rate, gb, n)
	if bytes <= 0 || n <= 0 {
		t.Fatalf("invalid estimate bytes=%d n=%d", bytes, n)
	}
	// Sanity: full-rate 7d retention is hundreds of GiB; operators must provision disk
	// or reduce sampling / enable downstream offload. Day files remain queryable by shard.
	if gb < 100 {
		t.Fatalf("capacity estimate too low (%.1f GiB); formula likely wrong", gb)
	}
	if ApproxBytesPerSample < 20 {
		t.Fatalf("ApproxBytesPerSample too low: %d", ApproxBytesPerSample)
	}
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	var b [12]byte
	pos := len(b)
	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}
	return string(b[pos:])
}
