// Package store provides an in-memory key-value store with TTL support.
package store

import (
	"sync"
	"time"
)

// Entry represents a value with optional expiration.
type Entry struct {
	Value     []byte
	ExpireAt  time.Time
	HasExpire bool
}

// Store represents an in-memory key-value store with TTL support.
// It is safe for concurrent use by multiple goroutines.
type Store struct {
	mu         sync.RWMutex
	data       map[string]*Entry
	sortedSets map[string]*SortedSet
	stopGC     chan struct{}
}

func cloneEntry(entry *Entry) *Entry {
	cloned := &Entry{
		ExpireAt:  entry.ExpireAt,
		HasExpire: entry.HasExpire,
	}
	if entry.Value != nil {
		cloned.Value = append([]byte(nil), entry.Value...)
	}
	return cloned
}

// New creates a new empty Store and starts the background expiration goroutine.
func New() *Store {
	s := &Store{
		data:       make(map[string]*Entry),
		sortedSets: make(map[string]*SortedSet),
		stopGC:     make(chan struct{}),
	}
	go s.gcLoop()
	return s
}

// gcLoop periodically removes expired keys.
func (s *Store) gcLoop() {
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()

	for {
		select {
		case <-s.stopGC:
			return
		case <-ticker.C:
			s.removeExpired()
		}
	}
}

// removeExpired removes all expired keys.
func (s *Store) removeExpired() {
	s.mu.Lock()
	defer s.mu.Unlock()

	now := time.Now()
	for key, entry := range s.data {
		if entry.HasExpire && now.After(entry.ExpireAt) {
			delete(s.data, key)
		}
	}
}

// Close stops the background GC goroutine.
func (s *Store) Close() {
	close(s.stopGC)
}

// isExpired checks if an entry is expired (must hold lock).
func (s *Store) isExpired(entry *Entry) bool {
	return entry.HasExpire && time.Now().After(entry.ExpireAt)
}

// Set stores a key-value pair in the store without expiration.
func (s *Store) Set(key string, value []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = &Entry{Value: append([]byte(nil), value...)}
}

// SetWithTTL stores a key-value pair with a TTL.
func (s *Store) SetWithTTL(key string, value []byte, ttl time.Duration) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = &Entry{
		Value:     append([]byte(nil), value...),
		ExpireAt:  time.Now().Add(ttl),
		HasExpire: true,
	}
}

// SetNX sets key to value if key does not exist. Returns true if set.
func (s *Store) SetNX(key string, value []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, exists := s.data[key]
	if exists && !s.isExpired(entry) {
		return false
	}

	s.data[key] = &Entry{Value: append([]byte(nil), value...)}
	return true
}

// Get retrieves a value by key from the store.
// Returns the value and true if found and not expired, nil and false otherwise.
func (s *Store) Get(key string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return nil, false
	}

	// Return a copy to prevent external mutation
	result := make([]byte, len(entry.Value))
	copy(result, entry.Value)
	return result, true
}

// Delete removes a key from the store.
// Returns true if the key existed, false otherwise.
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return false
	}
	delete(s.data, key)
	return true
}

// Exists checks if a key exists and is not expired.
func (s *Store) Exists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	return ok && !s.isExpired(entry)
}

// Expire sets a TTL on an existing key. Returns true if successful.
func (s *Store) Expire(key string, ttl time.Duration) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return false
	}

	entry.ExpireAt = time.Now().Add(ttl)
	entry.HasExpire = true
	return true
}

// TTL returns the remaining TTL for a key.
// Returns -2 if key doesn't exist, -1 if no TTL, otherwise TTL in duration.
func (s *Store) TTL(key string) time.Duration {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return -2 * time.Second
	}

	if !entry.HasExpire {
		return -1 * time.Second
	}

	remaining := time.Until(entry.ExpireAt)
	if remaining < 0 {
		return -2 * time.Second
	}
	return remaining
}

// Persist removes the TTL from a key. Returns true if successful.
func (s *Store) Persist(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return false
	}

	if !entry.HasExpire {
		return false
	}

	entry.HasExpire = false
	entry.ExpireAt = time.Time{}
	return true
}

// Keys returns all non-expired keys in the store.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	keys := make([]string, 0, len(s.data))
	for k, entry := range s.data {
		if !s.isExpired(entry) {
			keys = append(keys, k)
		}
	}
	return keys
}

// Size returns the number of non-expired keys in the store.
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	count := 0
	for _, entry := range s.data {
		if !s.isExpired(entry) {
			count++
		}
	}
	return count
}

// Clear removes all keys from the store.
func (s *Store) Clear() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data = make(map[string]*Entry)
	s.sortedSets = make(map[string]*SortedSet)
}

// Append appends value to existing key. Returns new length.
func (s *Store) Append(key string, value []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		s.data[key] = &Entry{Value: append([]byte(nil), value...)}
		return len(value)
	}

	entry.Value = append(entry.Value, value...)
	return len(entry.Value)
}

// StrLen returns the length of the value at key.
func (s *Store) StrLen(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return 0
	}
	return len(entry.Value)
}

// IncrBy increments the integer value of key by delta.
// Returns the new value and any error.
func (s *Store) IncrBy(key string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	entry, ok := s.data[key]
	var currentVal int64 = 0

	if ok && !s.isExpired(entry) {
		// Parse existing value
		val, err := parseInt64(entry.Value)
		if err != nil {
			return 0, err
		}
		currentVal = val
	}

	newVal := currentVal + delta
	s.data[key] = &Entry{Value: formatInt64(newVal)}
	return newVal, nil
}

// GetEntry returns the raw entry for a key (used by engine for WAL).
func (s *Store) GetEntry(key string) (*Entry, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	entry, ok := s.data[key]
	if !ok || s.isExpired(entry) {
		return nil, false
	}
	return cloneEntry(entry), true
}

// SetEntry sets a raw entry (used by engine for recovery).
func (s *Store) SetEntry(key string, entry *Entry) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.data[key] = cloneEntry(entry)
}

// Helper functions for integer parsing
func parseInt64(b []byte) (int64, error) {
	var result int64
	negative := false
	i := 0

	if len(b) == 0 {
		return 0, &intError{"value is not an integer"}
	}

	if b[0] == '-' {
		negative = true
		i++
	}

	if i >= len(b) {
		return 0, &intError{"value is not an integer"}
	}

	for ; i < len(b); i++ {
		if b[i] < '0' || b[i] > '9' {
			return 0, &intError{"value is not an integer"}
		}
		result = result*10 + int64(b[i]-'0')
	}

	if negative {
		result = -result
	}
	return result, nil
}

func formatInt64(n int64) []byte {
	if n == 0 {
		return []byte("0")
	}

	negative := n < 0
	if negative {
		n = -n
	}

	var buf [20]byte
	i := len(buf)

	for n > 0 {
		i--
		buf[i] = byte(n%10) + '0'
		n /= 10
	}

	if negative {
		i--
		buf[i] = '-'
	}

	return append([]byte(nil), buf[i:]...)
}

type intError struct {
	msg string
}

func (e *intError) Error() string {
	return e.msg
}

// ========================
// Sorted Set Operations
// ========================

// ZAdd adds members to a sorted set. Returns number of NEW members added.
func (s *Store) ZAdd(key string, members ...ScoredMember) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		zset = NewSortedSet()
		s.sortedSets[key] = zset
	}
	return zset.Add(members...)
}

// ZScore returns the score of a member in a sorted set.
func (s *Store) ZScore(key, member string) (float64, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0, false
	}
	return zset.Score(member)
}

// ZRem removes members from a sorted set. Returns number removed.
func (s *Store) ZRem(key string, members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0
	}
	removed := zset.Remove(members...)
	if zset.Card() == 0 {
		delete(s.sortedSets, key)
	}
	return removed
}

// ZCard returns the cardinality of a sorted set.
func (s *Store) ZCard(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0
	}
	return zset.Card()
}

// ZRank returns the rank of a member (0-based, ascending).
func (s *Store) ZRank(key, member string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return -1, false
	}
	return zset.Rank(member)
}

// ZRevRank returns the rank of a member (0-based, descending).
func (s *Store) ZRevRank(key, member string) (int, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return -1, false
	}
	return zset.RevRank(member)
}

// ZRange returns members by rank range.
func (s *Store) ZRange(key string, start, stop int, withScores bool) []ScoredMember {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	return zset.Range(start, stop, withScores)
}

// ZRevRange returns members by rank range in reverse order.
func (s *Store) ZRevRange(key string, start, stop int, withScores bool) []ScoredMember {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	return zset.RevRange(start, stop, withScores)
}

// ZRangeByScore returns members with scores in range.
func (s *Store) ZRangeByScore(key string, min, max float64, withScores bool, offset, count int) []ScoredMember {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	return zset.RangeByScore(min, max, withScores, offset, count)
}

// ZRevRangeByScore returns members with scores in range, descending.
func (s *Store) ZRevRangeByScore(key string, max, min float64, withScores bool, offset, count int) []ScoredMember {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	return zset.RevRangeByScore(max, min, withScores, offset, count)
}

// ZCount returns count of members with scores in range.
func (s *Store) ZCount(key string, min, max float64) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0
	}
	return zset.Count(min, max)
}

// ZIncrBy increments score of member. Returns new score.
func (s *Store) ZIncrBy(key, member string, increment float64) float64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		zset = NewSortedSet()
		s.sortedSets[key] = zset
	}
	return zset.IncrBy(member, increment)
}

// ZRemRangeByRank removes members by rank range.
func (s *Store) ZRemRangeByRank(key string, start, stop int) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0
	}
	removed := zset.RemoveRangeByRank(start, stop)
	if zset.Card() == 0 {
		delete(s.sortedSets, key)
	}
	return removed
}

// ZRemRangeByScore removes members by score range.
func (s *Store) ZRemRangeByScore(key string, min, max float64) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return 0
	}
	removed := zset.RemoveRangeByScore(min, max)
	if zset.Card() == 0 {
		delete(s.sortedSets, key)
	}
	return removed
}

// ZPopMin removes and returns lowest-scoring members.
func (s *Store) ZPopMin(key string, count int) []ScoredMember {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	result := zset.PopMin(count)
	if zset.Card() == 0 {
		delete(s.sortedSets, key)
	}
	return result
}

// ZPopMax removes and returns highest-scoring members.
func (s *Store) ZPopMax(key string, count int) []ScoredMember {
	s.mu.Lock()
	defer s.mu.Unlock()

	zset, exists := s.sortedSets[key]
	if !exists {
		return nil
	}
	result := zset.PopMax(count)
	if zset.Card() == 0 {
		delete(s.sortedSets, key)
	}
	return result
}

// ZExists checks if a sorted set exists.
func (s *Store) ZExists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.sortedSets[key]
	return exists
}
