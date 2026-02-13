package engine

import (
	"path/filepath"
	"sort"
	"testing"

	"github.com/flashdb/flashdb/internal/store"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ─── Hash engine tests ─────────────────────────────────────────────────────

func TestEngine_HSetAndHGet(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	n, err := e.HSet("user:1", store.HashFieldValue{Field: "name", Value: []byte("alice")}, store.HashFieldValue{Field: "age", Value: []byte("30")})
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	val, ok := e.HGet("user:1", "name")
	assert.True(t, ok)
	assert.Equal(t, []byte("alice"), val)

	_, ok = e.HGet("user:1", "missing")
	assert.False(t, ok)

	_, ok = e.HGet("nokey", "name")
	assert.False(t, ok)
}

func TestEngine_HDel(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.HSet("h", store.HashFieldValue{Field: "a", Value: []byte("1")}, store.HashFieldValue{Field: "b", Value: []byte("2")})

	n, err := e.HDel("h", "a", "missing")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 1, e.HLen("h"))
}

func TestEngine_HGetAll(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.HSet("h", store.HashFieldValue{Field: "x", Value: []byte("1")}, store.HashFieldValue{Field: "y", Value: []byte("2")})
	pairs := e.HGetAll("h")
	assert.Len(t, pairs, 2)
}

func TestEngine_HKeysVals(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.HSet("h", store.HashFieldValue{Field: "a", Value: []byte("1")}, store.HashFieldValue{Field: "b", Value: []byte("2")})
	keys := e.HKeys("h")
	assert.Len(t, keys, 2)
	vals := e.HVals("h")
	assert.Len(t, vals, 2)
}

func TestEngine_HIncrBy(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	v, err := e.HIncrBy("h", "counter", 5)
	require.NoError(t, err)
	assert.Equal(t, int64(5), v)

	v, err = e.HIncrBy("h", "counter", 3)
	require.NoError(t, err)
	assert.Equal(t, int64(8), v)
}

func TestEngine_HIncrByFloat(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	v, err := e.HIncrByFloat("h", "pi", 3.14)
	require.NoError(t, err)
	assert.InDelta(t, 3.14, v, 0.001)
}

func TestEngine_HSetNX(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	ok, err := e.HSetNX("h", "f", []byte("val"))
	require.NoError(t, err)
	assert.True(t, ok)

	ok, err = e.HSetNX("h", "f", []byte("other"))
	require.NoError(t, err)
	assert.False(t, ok)

	val, _ := e.HGet("h", "f")
	assert.Equal(t, []byte("val"), val)
}

func TestEngine_HExists(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.HSet("h", store.HashFieldValue{Field: "a", Value: []byte("1")})
	assert.True(t, e.HExists("h", "a"))
	assert.False(t, e.HExists("h", "b"))
	assert.False(t, e.HExists("nokey", "a"))
}

// ─── List engine tests ──────────────────────────────────────────────────────

func TestEngine_LPushRPush(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	n, err := e.RPush("list", []byte("a"), []byte("b"))
	require.NoError(t, err)
	assert.Equal(t, 2, n)

	n, err = e.LPush("list", []byte("z"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	assert.Equal(t, 3, e.LLen("list"))
}

func TestEngine_LPopRPop(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("c"))

	val, ok, err := e.LPop("list")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("a"), val)

	val, ok, err = e.RPop("list")
	require.NoError(t, err)
	assert.True(t, ok)
	assert.Equal(t, []byte("c"), val)

	// Pop from empty key
	_, ok, err = e.LPop("nokey")
	require.NoError(t, err)
	assert.False(t, ok)
}

func TestEngine_LIndex(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("c"))

	val, ok := e.LIndex("list", 1)
	assert.True(t, ok)
	assert.Equal(t, []byte("b"), val)

	val, ok = e.LIndex("list", -1)
	assert.True(t, ok)
	assert.Equal(t, []byte("c"), val)

	_, ok = e.LIndex("list", 99)
	assert.False(t, ok)
}

func TestEngine_LSet(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("c"))

	err = e.LSet("list", 1, []byte("x"))
	require.NoError(t, err)

	val, _ := e.LIndex("list", 1)
	assert.Equal(t, []byte("x"), val)

	err = e.LSet("list", 99, []byte("y"))
	assert.Error(t, err)
}

func TestEngine_LRange(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("c"), []byte("d"))

	items := e.LRange("list", 0, -1)
	assert.Len(t, items, 4)

	items = e.LRange("list", 1, 2)
	assert.Len(t, items, 2)
	assert.Equal(t, []byte("b"), items[0])
}

func TestEngine_LInsert(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("c"))

	n, err := e.LInsert("list", true, []byte("c"), []byte("b"))
	require.NoError(t, err)
	assert.Equal(t, 3, n)

	items := e.LRange("list", 0, -1)
	assert.Equal(t, []byte("b"), items[1])
}

func TestEngine_LRem(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("a"), []byte("c"))

	n, err := e.LRem("list", 1, []byte("a"))
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 3, e.LLen("list"))
}

func TestEngine_LTrim(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.RPush("list", []byte("a"), []byte("b"), []byte("c"), []byte("d"))

	err = e.LTrim("list", 1, 2)
	require.NoError(t, err)
	assert.Equal(t, 2, e.LLen("list"))

	items := e.LRange("list", 0, -1)
	assert.Equal(t, []byte("b"), items[0])
	assert.Equal(t, []byte("c"), items[1])
}

// ─── Set engine tests ───────────────────────────────────────────────────────

func TestEngine_SAddAndSCard(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	n, err := e.SAdd("set", "a", "b", "c")
	require.NoError(t, err)
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, e.SCard("set"))

	// Duplicate
	n, err = e.SAdd("set", "a", "d")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
}

func TestEngine_SRem(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("set", "a", "b", "c")
	n, err := e.SRem("set", "a", "missing")
	require.NoError(t, err)
	assert.Equal(t, 1, n)
	assert.Equal(t, 2, e.SCard("set"))
}

func TestEngine_SIsMember(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("set", "x")
	assert.True(t, e.SIsMember("set", "x"))
	assert.False(t, e.SIsMember("set", "y"))
	assert.False(t, e.SIsMember("nokey", "x"))
}

func TestEngine_SMembers(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("set", "a", "b", "c")
	members := e.SMembers("set")
	sort.Strings(members)
	assert.Equal(t, []string{"a", "b", "c"}, members)
}

func TestEngine_SRandMember(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("set", "a", "b", "c")
	members := e.SRandMember("set", 2)
	assert.Len(t, members, 2)
}

func TestEngine_SPop(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("set", "a", "b", "c")
	popped, err := e.SPop("set", 2)
	require.NoError(t, err)
	assert.Len(t, popped, 2)
	assert.Equal(t, 1, e.SCard("set"))
}

func TestEngine_SInter(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("s1", "a", "b", "c")
	e.SAdd("s2", "b", "c", "d")

	result := e.SInter("s1", "s2")
	sort.Strings(result)
	assert.Equal(t, []string{"b", "c"}, result)
}

func TestEngine_SUnion(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("s1", "a", "b")
	e.SAdd("s2", "b", "c")

	result := e.SUnion("s1", "s2")
	sort.Strings(result)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestEngine_SDiff(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.SAdd("s1", "a", "b", "c")
	e.SAdd("s2", "b")

	result := e.SDiff("s1", "s2")
	sort.Strings(result)
	assert.Equal(t, []string{"a", "c"}, result)
}

// ─── KeyType tests ──────────────────────────────────────────────────────────

func TestEngine_KeyType_NewTypes(t *testing.T) {
	e, err := New(filepath.Join(t.TempDir(), "test.wal"))
	require.NoError(t, err)
	defer e.Close()

	e.Set("str", []byte("hello"))
	e.HSet("hash", store.HashFieldValue{Field: "f", Value: []byte("v")})
	e.RPush("list", []byte("a"))
	e.SAdd("set", "x")

	assert.Equal(t, "string", e.KeyType("str"))
	assert.Equal(t, "hash", e.KeyType("hash"))
	assert.Equal(t, "list", e.KeyType("list"))
	assert.Equal(t, "set", e.KeyType("set"))
	assert.Equal(t, "none", e.KeyType("missing"))
}

// ─── WAL recovery tests ─────────────────────────────────────────────────────

func TestEngine_HashRecovery(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "test.wal")

	// Write data
	e, err := New(walPath)
	require.NoError(t, err)
	e.HSet("h", store.HashFieldValue{Field: "a", Value: []byte("1")}, store.HashFieldValue{Field: "b", Value: []byte("2")})
	e.HDel("h", "b")
	e.Close()

	// Recover
	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	val, ok := e2.HGet("h", "a")
	assert.True(t, ok)
	assert.Equal(t, []byte("1"), val)

	_, ok = e2.HGet("h", "b")
	assert.False(t, ok)
	assert.Equal(t, 1, e2.HLen("h"))
}

func TestEngine_ListRecovery(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)
	e.RPush("l", []byte("a"), []byte("b"), []byte("c"))
	e.LPop("l")
	e.Close()

	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	assert.Equal(t, 2, e2.LLen("l"))
	items := e2.LRange("l", 0, -1)
	assert.Equal(t, []byte("b"), items[0])
	assert.Equal(t, []byte("c"), items[1])
}

func TestEngine_SetRecovery(t *testing.T) {
	dir := t.TempDir()
	walPath := filepath.Join(dir, "test.wal")

	e, err := New(walPath)
	require.NoError(t, err)
	e.SAdd("s", "a", "b", "c")
	e.SRem("s", "b")
	e.Close()

	e2, err := New(walPath)
	require.NoError(t, err)
	defer e2.Close()

	assert.Equal(t, 2, e2.SCard("s"))
	assert.True(t, e2.SIsMember("s", "a"))
	assert.False(t, e2.SIsMember("s", "b"))
	assert.True(t, e2.SIsMember("s", "c"))
}
