package store

import (
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestStore_SetAndGet(t *testing.T) {
	s := New()
	defer s.Close()

	s.Set("key1", []byte("value1"))
	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)

	val, ok = s.Get("nonexistent")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStore_Delete(t *testing.T) {
	s := New()
	defer s.Close()

	s.Set("key1", []byte("value1"))
	deleted := s.Delete("key1")
	assert.True(t, deleted)

	val, ok := s.Get("key1")
	assert.False(t, ok)
	assert.Nil(t, val)
}

func TestStore_TTL(t *testing.T) {
	s := New()
	defer s.Close()

	s.SetWithTTL("key1", []byte("value1"), 100*time.Millisecond)

	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)

	time.Sleep(150 * time.Millisecond)

	_, ok = s.Get("key1")
	assert.False(t, ok)
}

func TestStore_IncrBy(t *testing.T) {
	s := New()
	defer s.Close()

	val, err := s.IncrBy("counter", 5)
	assert.NoError(t, err)
	assert.Equal(t, int64(5), val)

	val, err = s.IncrBy("counter", 10)
	assert.NoError(t, err)
	assert.Equal(t, int64(15), val)
}

func TestStore_ConcurrentAccess(t *testing.T) {
	s := New()
	defer s.Close()
	var wg sync.WaitGroup

	for i := 0; i < 100; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			s.Set(string(rune('a'+i%26)), []byte("value"))
		}(i)
	}

	wg.Wait()
}

func TestStore_SetCopiesInput(t *testing.T) {
	s := New()
	defer s.Close()

	source := []byte("value1")
	s.Set("key1", source)
	source[0] = 'X'

	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
}

func TestStore_GetEntryReturnsCopy(t *testing.T) {
	s := New()
	defer s.Close()

	s.SetWithTTL("key1", []byte("value1"), 5*time.Second)
	entry, ok := s.GetEntry("key1")
	assert.True(t, ok)

	// Mutate returned entry and ensure store state is unaffected.
	entry.Value[0] = 'X'
	entry.HasExpire = false

	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
	assert.True(t, s.TTL("key1") > 0)
}

func TestStore_SetEntryCopiesInput(t *testing.T) {
	s := New()
	defer s.Close()

	entry := &Entry{
		Value:     []byte("value1"),
		ExpireAt:  time.Now().Add(5 * time.Second),
		HasExpire: true,
	}
	s.SetEntry("key1", entry)

	entry.Value[0] = 'X'
	entry.HasExpire = false

	val, ok := s.Get("key1")
	assert.True(t, ok)
	assert.Equal(t, []byte("value1"), val)
	assert.True(t, s.TTL("key1") > 0)
}
