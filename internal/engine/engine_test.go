package engine

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEngine_OpenAndClose(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)
	require.NotNil(t, e)

	err = e.Close()
	require.NoError(t, err)
}

func TestEngine_SetAndGet(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	err = e.Set("key1", []byte("value1"))
	require.NoError(t, err)

	val, found := e.Get("key1")
	assert.True(t, found)
	assert.Equal(t, []byte("value1"), val)
}

func TestEngine_Delete(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	e.Set("key1", []byte("value1"))

	deleted, err := e.Delete("key1")
	require.NoError(t, err)
	assert.True(t, deleted)

	_, found := e.Get("key1")
	assert.False(t, found)
}

func TestEngine_SetWithTTL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	err = e.SetWithTTL("key1", []byte("value1"), 100*time.Millisecond)
	require.NoError(t, err)

	val, found := e.Get("key1")
	assert.True(t, found)
	assert.Equal(t, []byte("value1"), val)

	time.Sleep(150 * time.Millisecond)

	_, found = e.Get("key1")
	assert.False(t, found)
}

func TestEngine_TTL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	err = e.SetWithTTL("key1", []byte("value1"), 5*time.Second)
	require.NoError(t, err)

	ttl := e.TTL("key1")
	assert.True(t, ttl > 0 && ttl <= 5)

	e.Set("key2", []byte("value2"))
	ttl = e.TTL("key2")
	assert.Equal(t, int64(-1), ttl)

	ttl = e.TTL("nonexistent")
	assert.Equal(t, int64(-2), ttl)
}

func TestEngine_Expire(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	e.Set("key1", []byte("value1"))

	ok, err := e.Expire("key1", 100*time.Millisecond)
	require.NoError(t, err)
	assert.True(t, ok)

	time.Sleep(150 * time.Millisecond)

	_, found := e.Get("key1")
	assert.False(t, found)
}

func TestEngine_Persist(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)

	e.SetWithTTL("key1", []byte("value1"), 1*time.Second)

	ok, err := e.Persist("key1")
	require.NoError(t, err)
	assert.True(t, ok)

	ttl := e.TTL("key1")
	assert.Equal(t, int64(-1), ttl)

	require.NoError(t, e.Close())

	// TTL removal must survive restart.
	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	ttl = e2.TTL("key1")
	assert.Equal(t, int64(-1), ttl)
}

func TestEngine_IncrBy(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	val, err := e.IncrBy("counter", 1)
	require.NoError(t, err)
	assert.Equal(t, int64(1), val)

	val, err = e.IncrBy("counter", 10)
	require.NoError(t, err)
	assert.Equal(t, int64(11), val)

	val, err = e.IncrBy("counter", -5)
	require.NoError(t, err)
	assert.Equal(t, int64(6), val)
}

func TestEngine_Append(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	length, err := e.Append("key1", []byte("Hello"))
	require.NoError(t, err)
	assert.Equal(t, 5, length)

	length, err = e.Append("key1", []byte(" World"))
	require.NoError(t, err)
	assert.Equal(t, 11, length)

	val, _ := e.Get("key1")
	assert.Equal(t, []byte("Hello World"), val)
}

func TestEngine_Keys(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	e.Set("user:1", []byte("alice"))
	e.Set("user:2", []byte("bob"))
	e.Set("item:1", []byte("widget"))

	keys := e.Keys()
	assert.Len(t, keys, 3)
}

func TestEngine_Recovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)
	e.Set("key1", []byte("value1"))
	e.Set("key2", []byte("value2"))
	e.Delete("key1")
	e.Close()

	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	_, found := e2.Get("key1")
	assert.False(t, found)

	val, found := e2.Get("key2")
	assert.True(t, found)
	assert.Equal(t, []byte("value2"), val)
}

func TestEngine_Stats(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	e.Set("key1", []byte("value1"))
	e.Set("key2", []byte("value2"))
	e.Get("key1")
	e.Get("nonexistent")
	e.Delete("key1")

	stats := e.GetStats()
	assert.True(t, stats.TotalWrites >= 2)
	assert.True(t, stats.TotalReads >= 2)
}

func TestEngine_FlushDB(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	e.Set("key1", []byte("value1"))
	e.Set("key2", []byte("value2"))

	e.Clear()

	assert.Equal(t, 0, e.Size())

	info, _ := os.Stat(walPath)
	assert.Equal(t, int64(0), info.Size())
}

func TestEngine_RenamePreservesTTL(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)

	err = e.SetWithTTL("old", []byte("value"), 5*time.Second)
	require.NoError(t, err)

	renamed, err := e.Rename("old", "new", false)
	require.NoError(t, err)
	assert.True(t, renamed)

	_, oldExists := e.Get("old")
	assert.False(t, oldExists)

	val, newExists := e.Get("new")
	assert.True(t, newExists)
	assert.Equal(t, []byte("value"), val)
	assert.True(t, e.PTTL("new") > 0)

	require.NoError(t, e.Close())

	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	val, newExists = e2.Get("new")
	assert.True(t, newExists)
	assert.Equal(t, []byte("value"), val)
	assert.True(t, e2.PTTL("new") > 0)
}

func TestEngine_RenameNX(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)
	defer e.Close()

	require.NoError(t, e.Set("old", []byte("value-old")))
	require.NoError(t, e.Set("new", []byte("value-new")))

	renamed, err := e.Rename("old", "new", true)
	require.NoError(t, err)
	assert.False(t, renamed)

	val, exists := e.Get("old")
	assert.True(t, exists)
	assert.Equal(t, []byte("value-old"), val)

	val, exists = e.Get("new")
	assert.True(t, exists)
	assert.Equal(t, []byte("value-new"), val)
}

func TestEngine_CopyPreservesTTLAndRecovery(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)

	err = e.SetWithTTL("source", []byte("value"), 5*time.Second)
	require.NoError(t, err)

	copied, err := e.Copy("source", "dest", false)
	require.NoError(t, err)
	assert.True(t, copied)

	sourceVal, sourceExists := e.Get("source")
	assert.True(t, sourceExists)
	assert.Equal(t, []byte("value"), sourceVal)
	assert.True(t, e.PTTL("source") > 0)

	destVal, destExists := e.Get("dest")
	assert.True(t, destExists)
	assert.Equal(t, []byte("value"), destVal)
	assert.True(t, e.PTTL("dest") > 0)

	copied, err = e.Copy("source", "dest", false)
	require.NoError(t, err)
	assert.False(t, copied)

	require.NoError(t, e.Close())

	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	destVal, destExists = e2.Get("dest")
	assert.True(t, destExists)
	assert.Equal(t, []byte("value"), destVal)
	assert.True(t, e2.PTTL("dest") > 0)
}
