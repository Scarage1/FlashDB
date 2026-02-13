package timeseries

import (
	"testing"
	"time"
)

func TestAdd_AutoTimestamp(t *testing.T) {
	s := New()
	defer s.Close()

	before := time.Now().UnixMilli()
	ts := s.Add("cpu", 0, 42.5, 0)
	after := time.Now().UnixMilli()

	if ts < before || ts > after {
		t.Fatalf("auto-timestamp %d not in range [%d, %d]", ts, before, after)
	}

	dp, ok := s.Get("cpu")
	if !ok {
		t.Fatal("expected to find key")
	}
	if dp.Value != 42.5 {
		t.Fatalf("expected 42.5, got %f", dp.Value)
	}
}

func TestAdd_ExplicitTimestamp(t *testing.T) {
	s := New()
	defer s.Close()

	s.Add("temp", 1000, 20.0, 0)
	s.Add("temp", 2000, 21.0, 0)
	s.Add("temp", 3000, 22.0, 0)

	dp, ok := s.Get("temp")
	if !ok || dp.Value != 22.0 {
		t.Fatalf("expected latest value 22.0, got %f", dp.Value)
	}
}

func TestAdd_OutOfOrder(t *testing.T) {
	s := New()
	defer s.Close()

	s.Add("temp", 3000, 30.0, 0)
	s.Add("temp", 1000, 10.0, 0)
	s.Add("temp", 2000, 20.0, 0)

	points, err := s.Range("temp", 0, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 3 {
		t.Fatalf("expected 3 points, got %d", len(points))
	}
	for i, expected := range []int64{1000, 2000, 3000} {
		if points[i].Timestamp != expected {
			t.Fatalf("point %d: expected ts %d, got %d", i, expected, points[i].Timestamp)
		}
	}
}

func TestRange(t *testing.T) {
	s := New()
	defer s.Close()

	for i := int64(0); i < 100; i++ {
		s.Add("sensor", i*100, float64(i), 0)
	}

	points, err := s.Range("sensor", 2000, 5000)
	if err != nil {
		t.Fatal(err)
	}
	if len(points) != 31 {
		t.Fatalf("expected 31 points, got %d", len(points))
	}
	if points[0].Timestamp != 2000 || points[len(points)-1].Timestamp != 5000 {
		t.Fatalf("unexpected range boundaries")
	}
}

func TestRange_NonExistentKey(t *testing.T) {
	s := New()
	defer s.Close()

	_, err := s.Range("missing", 0, 1000)
	if err == nil {
		t.Fatal("expected error for missing key")
	}
}

func TestGetInfo(t *testing.T) {
	s := New()
	defer s.Close()

	s.Add("mem", 100, 1.0, 30*time.Second)
	s.Add("mem", 200, 2.0, 30*time.Second)
	s.Add("mem", 300, 3.0, 30*time.Second)

	info, err := s.GetInfo("mem")
	if err != nil {
		t.Fatal(err)
	}
	if info.TotalSamples != 3 {
		t.Fatalf("expected 3 samples, got %d", info.TotalSamples)
	}
	if info.FirstTS != 100 || info.LastTS != 300 {
		t.Fatalf("unexpected timestamps: first=%d last=%d", info.FirstTS, info.LastTS)
	}
	if info.Retention != 30*time.Second {
		t.Fatalf("expected 30s retention, got %v", info.Retention)
	}
}

func TestDelete(t *testing.T) {
	s := New()
	defer s.Close()

	s.Add("k1", 1000, 1.0, 0)
	if !s.Delete("k1") {
		t.Fatal("expected true for existing key")
	}
	if s.Delete("k1") {
		t.Fatal("expected false for already deleted key")
	}
}

func TestKeys(t *testing.T) {
	s := New()
	defer s.Close()

	s.Add("a", 1, 1.0, 0)
	s.Add("b", 1, 2.0, 0)
	s.Add("c", 1, 3.0, 0)

	keys := s.Keys()
	if len(keys) != 3 {
		t.Fatalf("expected 3 keys, got %d", len(keys))
	}
}

func TestRetention(t *testing.T) {
	s := New()
	defer s.Close()

	now := time.Now().UnixMilli()
	s.Add("r", now-10000, 1.0, 5*time.Second)
	s.Add("r", now-3000, 2.0, 5*time.Second)
	s.Add("r", now, 3.0, 5*time.Second)

	s.removeExpired()

	info, err := s.GetInfo("r")
	if err != nil {
		t.Fatal(err)
	}
	if info.TotalSamples != 2 {
		t.Fatalf("expected 2 samples after retention, got %d", info.TotalSamples)
	}
}

func TestSize(t *testing.T) {
	s := New()
	defer s.Close()

	if s.Size() != 0 {
		t.Fatal("expected empty store")
	}
	s.Add("a", 1, 1.0, 0)
	s.Add("b", 1, 1.0, 0)
	if s.Size() != 2 {
		t.Fatalf("expected 2, got %d", s.Size())
	}
}
