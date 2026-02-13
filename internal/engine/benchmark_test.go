package engine

import (
	"fmt"
	"path/filepath"
	"strconv"
	"testing"

	"github.com/flashdb/flashdb/internal/store"
)

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func newBenchEngine(b *testing.B) *Engine {
	b.Helper()
	e, err := New(filepath.Join(b.TempDir(), "bench.wal"))
	if err != nil {
		b.Fatal(err)
	}
	b.Cleanup(func() { e.Close() })
	return e
}

// ---------------------------------------------------------------------------
// String benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSet(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Set("key", []byte("value"))
	}
}

func BenchmarkGet(b *testing.B) {
	e := newBenchEngine(b)
	e.Set("key", []byte("value"))
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Get("key")
	}
}

func BenchmarkSetGet(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Set("key", []byte("value"))
		e.Get("key")
	}
}

func BenchmarkIncrBy(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.IncrBy("counter", 1)
	}
}

// ---------------------------------------------------------------------------
// Hash benchmarks
// ---------------------------------------------------------------------------

func BenchmarkHSet(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.HSet("myhash", store.HashFieldValue{Field: "f1", Value: []byte("v1")})
	}
}

func BenchmarkHGet(b *testing.B) {
	e := newBenchEngine(b)
	e.HSet("myhash", store.HashFieldValue{Field: "f1", Value: []byte("v1")})
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.HGet("myhash", "f1")
	}
}

func BenchmarkHGetAll(b *testing.B) {
	e := newBenchEngine(b)
	for i := 0; i < 100; i++ {
		e.HSet("myhash", store.HashFieldValue{Field: fmt.Sprintf("f%d", i), Value: []byte("v")})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.HGetAll("myhash")
	}
}

// ---------------------------------------------------------------------------
// List benchmarks
// ---------------------------------------------------------------------------

func BenchmarkLPush(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.LPush("mylist", []byte("val"))
	}
}

func BenchmarkRPush(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.RPush("mylist", []byte("val"))
	}
}

func BenchmarkLRange(b *testing.B) {
	e := newBenchEngine(b)
	for i := 0; i < 1000; i++ {
		e.RPush("mylist", []byte("val"))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.LRange("mylist", 0, 99)
	}
}

// ---------------------------------------------------------------------------
// Set benchmarks
// ---------------------------------------------------------------------------

func BenchmarkSAdd(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.SAdd("myset", fmt.Sprintf("m%d", i))
	}
}

func BenchmarkSMembers(b *testing.B) {
	e := newBenchEngine(b)
	for i := 0; i < 100; i++ {
		e.SAdd("myset", fmt.Sprintf("m%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.SMembers("myset")
	}
}

func BenchmarkSIsMember(b *testing.B) {
	e := newBenchEngine(b)
	for i := 0; i < 100; i++ {
		e.SAdd("myset", fmt.Sprintf("m%d", i))
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.SIsMember("myset", "m50")
	}
}

// ---------------------------------------------------------------------------
// Sorted-set benchmarks
// ---------------------------------------------------------------------------

func BenchmarkZAdd(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ZAdd("zkey", store.ScoredMember{Member: fmt.Sprintf("m%d", i), Score: float64(i)})
	}
}

func BenchmarkZRangeByScore(b *testing.B) {
	e := newBenchEngine(b)
	for i := 0; i < 1000; i++ {
		e.ZAdd("zkey", store.ScoredMember{Member: fmt.Sprintf("m%d", i), Score: float64(i)})
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.ZRangeByScore("zkey", 0, 100, false, 0, -1)
	}
}

// ---------------------------------------------------------------------------
// Parallel / concurrent benchmarks
// ---------------------------------------------------------------------------

func BenchmarkParallelSet(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Set("key", []byte("value"))
		}
	})
}

func BenchmarkParallelGet(b *testing.B) {
	e := newBenchEngine(b)
	e.Set("key", []byte("value"))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Get("key")
		}
	})
}

func BenchmarkParallelMixed(b *testing.B) {
	e := newBenchEngine(b)
	e.Set("key", []byte("value"))
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			if i%2 == 0 {
				e.Get("key")
			} else {
				e.Set("key", []byte("value"))
			}
			i++
		}
	})
}

func BenchmarkParallelHSet(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			e.HSet("h", store.HashFieldValue{Field: strconv.Itoa(i), Value: []byte("v")})
			i++
		}
	})
}

func BenchmarkParallelSAdd(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			e.SAdd("s", strconv.Itoa(i))
			i++
		}
	})
}

func BenchmarkParallelLPush(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.LPush("l", []byte("v"))
		}
	})
}

func BenchmarkParallelZAdd(b *testing.B) {
	e := newBenchEngine(b)
	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		i := 0
		for pb.Next() {
			e.ZAdd("z", store.ScoredMember{Member: strconv.Itoa(i), Score: float64(i)})
			i++
		}
	})
}

// BenchmarkPipelineSimulation simulates pipelined SET+GET bursts.
func BenchmarkPipelineSimulation(b *testing.B) {
	e := newBenchEngine(b)
	const batchSize = 50
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		for j := 0; j < batchSize; j++ {
			k := strconv.Itoa(j)
			e.Set(k, []byte(k))
		}
		for j := 0; j < batchSize; j++ {
			e.Get(strconv.Itoa(j))
		}
	}
}
