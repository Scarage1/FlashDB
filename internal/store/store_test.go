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

	val, ok = s.Get("key1")
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
