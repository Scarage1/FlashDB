package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestHash_SetAndGet(t *testing.T) {
	h := NewHash()
	assert.True(t, h.Set("name", []byte("alice")))
	val, ok := h.Get("name")
	assert.True(t, ok)
	assert.Equal(t, []byte("alice"), val)

	// Overwrite returns false (not a new field)
	assert.False(t, h.Set("name", []byte("bob")))
	val, _ = h.Get("name")
	assert.Equal(t, []byte("bob"), val)

	_, ok = h.Get("missing")
	assert.False(t, ok)
}

func TestHash_Del(t *testing.T) {
	h := NewHash()
	h.Set("a", []byte("1"))
	h.Set("b", []byte("2"))
	h.Set("c", []byte("3"))

	n := h.Del("a", "c", "missing")
	assert.Equal(t, 2, n)
	assert.Equal(t, 1, h.Len())
}

func TestHash_GetAll(t *testing.T) {
	h := NewHash()
	h.Set("x", []byte("1"))
	h.Set("y", []byte("2"))

	pairs := h.GetAll()
	assert.Len(t, pairs, 2)
	m := make(map[string]string)
	for _, p := range pairs {
		m[p.Field] = string(p.Value)
	}
	assert.Equal(t, "1", m["x"])
	assert.Equal(t, "2", m["y"])
}

func TestHash_Keys_Vals(t *testing.T) {
	h := NewHash()
	h.Set("a", []byte("1"))
	h.Set("b", []byte("2"))

	keys := h.Keys()
	assert.Len(t, keys, 2)
	assert.ElementsMatch(t, []string{"a", "b"}, keys)

	vals := h.Vals()
	assert.Len(t, vals, 2)
}

func TestHash_Exists(t *testing.T) {
	h := NewHash()
	h.Set("key", []byte("val"))
	assert.True(t, h.Exists("key"))
	assert.False(t, h.Exists("nope"))
}

func TestHash_IncrBy(t *testing.T) {
	h := NewHash()

	// Increment non-existing field
	val, err := h.IncrBy("counter", 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), val)

	val, err = h.IncrBy("counter", 3)
	assert.NoError(t, err)
	assert.Equal(t, int64(8), val)

	// Non-integer value
	h.Set("str", []byte("notanumber"))
	_, err = h.IncrBy("str", 1)
	assert.Error(t, err)
}

func TestHash_IncrByFloat(t *testing.T) {
	h := NewHash()

	val, err := h.IncrByFloat("pi", 3.14)
	assert.NoError(t, err)
	assert.InDelta(t, 3.14, val, 0.001)

	val, err = h.IncrByFloat("pi", 0.001)
	assert.NoError(t, err)
	assert.InDelta(t, 3.141, val, 0.0001)
}

func TestHash_SetNX(t *testing.T) {
	h := NewHash()
	assert.True(t, h.SetNX("key", []byte("val")))
	assert.False(t, h.SetNX("key", []byte("other")))
	val, _ := h.Get("key")
	assert.Equal(t, []byte("val"), val)
}

func TestHash_Len(t *testing.T) {
	h := NewHash()
	assert.Equal(t, 0, h.Len())
	h.Set("a", []byte("1"))
	h.Set("b", []byte("2"))
	assert.Equal(t, 2, h.Len())
	h.Del("a")
	assert.Equal(t, 1, h.Len())
}
