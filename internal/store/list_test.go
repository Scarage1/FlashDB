package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestList_PushAndLen(t *testing.T) {
	l := NewList()
	assert.Equal(t, 0, l.Len())

	n := l.RPush([]byte("a"), []byte("b"), []byte("c"))
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, l.Len())

	n = l.LPush([]byte("z"))
	assert.Equal(t, 4, n)
}

func TestList_Pop(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("c"))

	val, ok := l.LPop()
	assert.True(t, ok)
	assert.Equal(t, []byte("a"), val)

	val, ok = l.RPop()
	assert.True(t, ok)
	assert.Equal(t, []byte("c"), val)

	l.LPop() // remove "b"
	_, ok = l.LPop()
	assert.False(t, ok)

	_, ok = l.RPop()
	assert.False(t, ok)
}

func TestList_Index(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("c"))

	val, ok := l.Index(0)
	assert.True(t, ok)
	assert.Equal(t, []byte("a"), val)

	val, ok = l.Index(-1)
	assert.True(t, ok)
	assert.Equal(t, []byte("c"), val)

	_, ok = l.Index(10)
	assert.False(t, ok)
}

func TestList_Set(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("c"))

	err := l.Set(1, []byte("x"))
	assert.NoError(t, err)

	val, _ := l.Index(1)
	assert.Equal(t, []byte("x"), val)

	err = l.Set(10, []byte("y"))
	assert.Error(t, err)

	err = l.Set(-1, []byte("z"))
	assert.NoError(t, err)
	val, _ = l.Index(2)
	assert.Equal(t, []byte("z"), val)
}

func TestList_Range(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e"))

	// Positive range
	items := l.Range(1, 3)
	assert.Len(t, items, 3)
	assert.Equal(t, []byte("b"), items[0])
	assert.Equal(t, []byte("d"), items[2])

	// Negative indices
	items = l.Range(-3, -1)
	assert.Len(t, items, 3)
	assert.Equal(t, []byte("c"), items[0])

	// Full range
	items = l.Range(0, -1)
	assert.Len(t, items, 5)

	// Out of range
	items = l.Range(10, 20)
	assert.Len(t, items, 0)
}

func TestList_InsertBefore(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("c"))

	n := l.Insert(true, []byte("c"), []byte("b"))
	assert.Equal(t, 3, n)

	items := l.Range(0, -1)
	assert.Equal(t, []byte("a"), items[0])
	assert.Equal(t, []byte("b"), items[1])
	assert.Equal(t, []byte("c"), items[2])
}

func TestList_InsertAfter(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("c"))

	n := l.Insert(false, []byte("a"), []byte("b"))
	assert.Equal(t, 3, n)

	items := l.Range(0, -1)
	assert.Equal(t, []byte("a"), items[0])
	assert.Equal(t, []byte("b"), items[1])
	assert.Equal(t, []byte("c"), items[2])
}

func TestList_InsertPivotNotFound(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"))

	n := l.Insert(true, []byte("missing"), []byte("b"))
	assert.Equal(t, -1, n)
}

func TestList_Rem(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("a"), []byte("c"), []byte("a"))

	// Remove 2 from head
	n := l.Rem(2, []byte("a"))
	assert.Equal(t, 2, n)
	assert.Equal(t, 3, l.Len())

	// Rebuild
	l2 := NewList()
	l2.RPush([]byte("a"), []byte("b"), []byte("a"), []byte("c"), []byte("a"))

	// Remove 1 from tail (negative count)
	n = l2.Rem(-1, []byte("a"))
	assert.Equal(t, 1, n)
	items := l2.Range(0, -1)
	assert.Equal(t, 4, len(items))
	assert.Equal(t, []byte("c"), items[3]) // last "a" removed, "c" is now last

	// Remove all (count=0)
	l3 := NewList()
	l3.RPush([]byte("x"), []byte("y"), []byte("x"), []byte("x"))
	n = l3.Rem(0, []byte("x"))
	assert.Equal(t, 3, n)
	assert.Equal(t, 1, l3.Len())
}

func TestList_Trim(t *testing.T) {
	l := NewList()
	l.RPush([]byte("a"), []byte("b"), []byte("c"), []byte("d"), []byte("e"))

	l.Trim(1, 3)
	assert.Equal(t, 3, l.Len())

	items := l.Range(0, -1)
	assert.Equal(t, []byte("b"), items[0])
	assert.Equal(t, []byte("d"), items[2])
}

func TestList_LPush_Order(t *testing.T) {
	l := NewList()
	// Redis LPUSH pushes each value to the head in order,
	// so LPUSH key a b c results in [c, b, a]
	l.LPush([]byte("a"), []byte("b"), []byte("c"))
	items := l.Range(0, -1)
	assert.Equal(t, []byte("c"), items[0])
	assert.Equal(t, []byte("b"), items[1])
	assert.Equal(t, []byte("a"), items[2])
}
