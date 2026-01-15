package engine

import (
	"path/filepath"
	"testing"
)

func BenchmarkSet(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Set("key", []byte("value"))
	}
}

func BenchmarkGet(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	e.Set("key", []byte("value"))

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Get("key")
	}
}

func BenchmarkSetGet(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.Set("key", []byte("value"))
		e.Get("key")
	}
}

func BenchmarkIncrBy(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		e.IncrBy("counter", 1)
	}
}

func BenchmarkParallelSet(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Set("key", []byte("value"))
		}
	})
}

func BenchmarkParallelGet(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

	e.Set("key", []byte("value"))

	b.ResetTimer()
	b.RunParallel(func(pb *testing.PB) {
		for pb.Next() {
			e.Get("key")
		}
	})
}

func BenchmarkParallelMixed(b *testing.B) {
	tmpDir := b.TempDir()
	walPath := filepath.Join(tmpDir, "bench.wal")
	e, _ := New(walPath)
	defer e.Close()

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
