package wal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWAL_OpenAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)
	require.NotNil(t, w)

	err = w.Close()
	require.NoError(t, err)

	_, err = os.Stat(walPath)
	assert.NoError(t, err)
}

func TestWAL_AppendAndReadAll(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)
	defer w.Close()

	records := []Record{
		{Type: OpSet, Key: []byte("key1"), Value: []byte("value1")},
		{Type: OpSetWithTTL, Key: []byte("key2"), Value: []byte("value2"), ExpireAt: 1234567890000},
		{Type: OpDelete, Key: []byte("key1"), Value: nil},
	}

	for _, rec := range records {
		err := w.Append(rec)
		require.NoError(t, err)
	}

	readRecords, err := w.ReadAll()
	require.NoError(t, err)
	require.Len(t, readRecords, 3)

	assert.Equal(t, OpSet, readRecords[0].Type)
	assert.Equal(t, []byte("key1"), readRecords[0].Key)

	assert.Equal(t, OpSetWithTTL, readRecords[1].Type)
	assert.Equal(t, int64(1234567890000), readRecords[1].ExpireAt)

	assert.Equal(t, OpDelete, readRecords[2].Type)
}

func TestWAL_Recovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)

	err = w.Append(Record{Type: OpSet, Key: []byte("key1"), Value: []byte("value1")})
	require.NoError(t, err)

	w.Close()

	w2, err := Open(walPath)
	require.NoError(t, err)
	defer w2.Close()

	records, err := w2.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1)
	assert.Equal(t, []byte("key1"), records[0].Key)
}

func TestWAL_PartialRecord(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)

	err = w.Append(Record{Type: OpSet, Key: []byte("key1"), Value: []byte("value1")})
	require.NoError(t, err)
	w.Close()

	// Append partial data
	f, err := os.OpenFile(walPath, os.O_APPEND|os.O_WRONLY, 0644)
	require.NoError(t, err)
	f.Write([]byte{0x01, 0x02, 0x03})
	f.Close()

	w2, err := Open(walPath)
	require.NoError(t, err)
	defer w2.Close()

	records, err := w2.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 1)
}

func TestWAL_Clear(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)
	defer w.Close()

	err = w.Append(Record{Type: OpSet, Key: []byte("key1"), Value: []byte("value1")})
	require.NoError(t, err)

	err = w.Clear()
	require.NoError(t, err)

	records, err := w.ReadAll()
	require.NoError(t, err)
	assert.Empty(t, records)
}

func TestWAL_AppendNoSyncAndSync(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	w, err := Open(walPath)
	require.NoError(t, err)
	defer w.Close()

	// Write several records without syncing
	for i := 0; i < 5; i++ {
		err = w.AppendNoSync(Record{Type: OpSet, Key: []byte("k"), Value: []byte("v")})
		require.NoError(t, err)
	}

	// Explicit sync
	require.NoError(t, w.Sync())

	records, err := w.ReadAll()
	require.NoError(t, err)
	require.Len(t, records, 5)
}

// ---------------------------------------------------------------------------
// Benchmarks
// ---------------------------------------------------------------------------

func BenchmarkWALAppend(b *testing.B) {
	w, _ := Open(filepath.Join(b.TempDir(), "bench.wal"))
	defer w.Close()
	rec := Record{Type: OpSet, Key: []byte("key"), Value: []byte("value")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.Append(rec)
	}
}

func BenchmarkWALAppendNoSync(b *testing.B) {
	w, _ := Open(filepath.Join(b.TempDir(), "bench.wal"))
	defer w.Close()
	rec := Record{Type: OpSet, Key: []byte("key"), Value: []byte("value")}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.AppendNoSync(rec)
	}
	w.Sync()
}

func BenchmarkWALAppendBatch(b *testing.B) {
	w, _ := Open(filepath.Join(b.TempDir(), "bench.wal"))
	defer w.Close()
	batch := make([]Record, 50)
	for i := range batch {
		batch[i] = Record{Type: OpSet, Key: []byte("key"), Value: []byte("value")}
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		w.AppendBatch(batch)
	}
}
