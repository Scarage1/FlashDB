// Package store provides an in-memory key-value store with TTL support.
package store

import (
	"fmt"
	"math/rand"
	"strconv"
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
	hashes     map[string]*Hash
	lists      map[string]*List
	sets       map[string]*Set
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
		hashes:     make(map[string]*Hash),
		lists:      make(map[string]*List),
		sets:       make(map[string]*Set),
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

// removeExpired uses Redis-style sampling to probabilistically remove expired keys.
// Algorithm: sample up to 20 random keys. Delete expired ones.
// If more than 25% of the sample was expired, repeat immediately (up to a bounded number of rounds).
// This avoids the O(N) full scan under write lock that previously blocked all operations.
func (s *Store) removeExpired() {
	const (
		sampleSize   = 20
		maxRounds    = 4
		expiredRatio = 0.25
	)

	for round := 0; round < maxRounds; round++ {
		s.mu.Lock()

		n := len(s.data)
		if n == 0 {
			s.mu.Unlock()
			return
		}

		// Collect all keys into a slice for random sampling.
		// For very large datasets this could be optimized with a separate expiry index,
		// but for now a random sample from the key space is adequate.
		now := time.Now()
		sampled := 0
		expired := 0

		// Use map iteration which is pseudo-random in Go
		for key, entry := range s.data {
			if sampled >= sampleSize {
				break
			}
			sampled++
			if entry.HasExpire && now.After(entry.ExpireAt) {
				delete(s.data, key)
				expired++
			}
		}

		s.mu.Unlock()

		// If fewer than 25% of sampled keys were expired, stop
		if sampled == 0 || float64(expired)/float64(sampled) < expiredRatio {
			return
		}
	}
}

// expiredKeySample picks up to n random keys that have expiration set.
// This is a helper for future advanced GC strategies.
func (s *Store) expiredKeySample(n int) []string {
	keys := make([]string, 0, n)
	for k, entry := range s.data {
		if entry.HasExpire {
			keys = append(keys, k)
			if len(keys) >= n {
				break
			}
		}
	}
	// Shuffle to avoid map iteration bias
	rand.Shuffle(len(keys), func(i, j int) {
		keys[i], keys[j] = keys[j], keys[i]
	})
	return keys
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
	s.hashes = make(map[string]*Hash)
	s.lists = make(map[string]*List)
	s.sets = make(map[string]*Set)
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

// ========================
// Float Helpers
// ========================

func parseFloat64(b []byte) (float64, error) {
	return strconv.ParseFloat(string(b), 64)
}

func formatFloat64(f float64) []byte {
	return []byte(strconv.FormatFloat(f, 'f', -1, 64))
}

// ========================
// Hash Operations
// ========================

// HSet sets field(s) in a hash. Returns number of new fields added.
func (s *Store) HSet(key string, fields ...HashFieldValue) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, exists := s.hashes[key]
	if !exists {
		h = NewHash()
		s.hashes[key] = h
	}
	added := 0
	for _, fv := range fields {
		if h.Set(fv.Field, fv.Value) {
			added++
		}
	}
	return added
}

// HGet returns the value of a hash field.
func (s *Store) HGet(key, field string) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return nil, false
	}
	return h.Get(field)
}

// HDel removes field(s) from a hash. Returns number removed.
func (s *Store) HDel(key string, fields ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, exists := s.hashes[key]
	if !exists {
		return 0
	}
	removed := h.Del(fields...)
	if h.Len() == 0 {
		delete(s.hashes, key)
	}
	return removed
}

// HExists returns whether a field exists in a hash.
func (s *Store) HExists(key, field string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return false
	}
	return h.Exists(field)
}

// HLen returns the number of fields in a hash.
func (s *Store) HLen(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return 0
	}
	return h.Len()
}

// HGetAll returns all field-value pairs in a hash.
func (s *Store) HGetAll(key string) []HashFieldValue {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return nil
	}
	return h.GetAll()
}

// HKeys returns all field names in a hash.
func (s *Store) HKeys(key string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return nil
	}
	return h.Keys()
}

// HVals returns all values in a hash.
func (s *Store) HVals(key string) [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	h, exists := s.hashes[key]
	if !exists {
		return nil
	}
	return h.Vals()
}

// HIncrBy increments integer value of a hash field.
func (s *Store) HIncrBy(key, field string, delta int64) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, exists := s.hashes[key]
	if !exists {
		h = NewHash()
		s.hashes[key] = h
	}
	return h.IncrBy(field, delta)
}

// HIncrByFloat increments float value of a hash field.
func (s *Store) HIncrByFloat(key, field string, delta float64) (float64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, exists := s.hashes[key]
	if !exists {
		h = NewHash()
		s.hashes[key] = h
	}
	return h.IncrByFloat(field, delta)
}

// HSetNX sets a field only if it does not exist. Returns true if set.
func (s *Store) HSetNX(key, field string, value []byte) bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	h, exists := s.hashes[key]
	if !exists {
		h = NewHash()
		s.hashes[key] = h
	}
	return h.SetNX(field, value)
}

// HashExists checks if a hash key exists.
func (s *Store) HashExists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.hashes[key]
	return exists
}

// ========================
// List Operations
// ========================

// LPush prepends values to a list. Returns new length.
func (s *Store) LPush(key string, values ...[]byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		l = NewList()
		s.lists[key] = l
	}
	return l.LPush(values...)
}

// RPush appends values to a list. Returns new length.
func (s *Store) RPush(key string, values ...[]byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		l = NewList()
		s.lists[key] = l
	}
	return l.RPush(values...)
}

// LPop removes and returns the first element.
func (s *Store) LPop(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return nil, false
	}
	val, ok := l.LPop()
	if l.Len() == 0 {
		delete(s.lists, key)
	}
	return val, ok
}

// RPop removes and returns the last element.
func (s *Store) RPop(key string) ([]byte, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return nil, false
	}
	val, ok := l.RPop()
	if l.Len() == 0 {
		delete(s.lists, key)
	}
	return val, ok
}

// LLen returns the length of a list.
func (s *Store) LLen(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, exists := s.lists[key]
	if !exists {
		return 0
	}
	return l.Len()
}

// LIndex returns the element at the given index.
func (s *Store) LIndex(key string, index int) ([]byte, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, exists := s.lists[key]
	if !exists {
		return nil, false
	}
	return l.Index(index)
}

// LSet sets the element at index.
func (s *Store) LSet(key string, index int, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return fmt.Errorf("no such key")
	}
	return l.Set(index, value)
}

// LRange returns elements from start to stop.
func (s *Store) LRange(key string, start, stop int) [][]byte {
	s.mu.RLock()
	defer s.mu.RUnlock()

	l, exists := s.lists[key]
	if !exists {
		return nil
	}
	return l.Range(start, stop)
}

// LInsert inserts value before/after pivot.
func (s *Store) LInsert(key string, before bool, pivot, value []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return 0
	}
	return l.Insert(before, pivot, value)
}

// LRem removes count occurrences of value.
func (s *Store) LRem(key string, count int, value []byte) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return 0
	}
	removed := l.Rem(count, value)
	if l.Len() == 0 {
		delete(s.lists, key)
	}
	return removed
}

// LTrim trims the list to elements between start and stop.
func (s *Store) LTrim(key string, start, stop int) {
	s.mu.Lock()
	defer s.mu.Unlock()

	l, exists := s.lists[key]
	if !exists {
		return
	}
	l.Trim(start, stop)
	if l.Len() == 0 {
		delete(s.lists, key)
	}
}

// ListExists checks if a list key exists.
func (s *Store) ListExists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.lists[key]
	return exists
}

// ========================
// Set Operations
// ========================

// SAdd adds members to a set. Returns number added.
func (s *Store) SAdd(key string, members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, exists := s.sets[key]
	if !exists {
		set = NewSet()
		s.sets[key] = set
	}
	return set.Add(members...)
}

// SRem removes members from a set. Returns number removed.
func (s *Store) SRem(key string, members ...string) int {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, exists := s.sets[key]
	if !exists {
		return 0
	}
	removed := set.Rem(members...)
	if set.Card() == 0 {
		delete(s.sets, key)
	}
	return removed
}

// SIsMember returns whether a member exists in a set.
func (s *Store) SIsMember(key, member string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, exists := s.sets[key]
	if !exists {
		return false
	}
	return set.IsMember(member)
}

// SCard returns the cardinality of a set.
func (s *Store) SCard(key string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, exists := s.sets[key]
	if !exists {
		return 0
	}
	return set.Card()
}

// SMembers returns all members of a set.
func (s *Store) SMembers(key string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, exists := s.sets[key]
	if !exists {
		return nil
	}
	return set.Members()
}

// SRandMember returns random member(s) from a set.
func (s *Store) SRandMember(key string, count int) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	set, exists := s.sets[key]
	if !exists {
		return nil
	}
	return set.RandMember(count)
}

// SPop removes and returns random member(s).
func (s *Store) SPop(key string, count int) []string {
	s.mu.Lock()
	defer s.mu.Unlock()

	set, exists := s.sets[key]
	if !exists {
		return nil
	}
	result := set.Pop(count)
	if set.Card() == 0 {
		delete(s.sets, key)
	}
	return result
}

// SInter returns intersection of multiple sets.
func (s *Store) SInter(keys ...string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(keys) == 0 {
		return nil
	}

	first, exists := s.sets[keys[0]]
	if !exists {
		return nil
	}

	others := make([]*Set, 0, len(keys)-1)
	for _, key := range keys[1:] {
		other, exists := s.sets[key]
		if !exists {
			return nil // Intersection with empty set is empty
		}
		others = append(others, other)
	}
	return first.Inter(others...)
}

// SUnion returns union of multiple sets.
func (s *Store) SUnion(keys ...string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(keys) == 0 {
		return nil
	}

	first, exists := s.sets[keys[0]]
	if !exists {
		first = NewSet() // Treat missing key as empty set
	}

	others := make([]*Set, 0, len(keys)-1)
	for _, key := range keys[1:] {
		other, exists := s.sets[key]
		if !exists {
			continue
		}
		others = append(others, other)
	}
	return first.Union(others...)
}

// SDiff returns members in first set not in any others.
func (s *Store) SDiff(keys ...string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if len(keys) == 0 {
		return nil
	}

	first, exists := s.sets[keys[0]]
	if !exists {
		return nil
	}

	others := make([]*Set, 0, len(keys)-1)
	for _, key := range keys[1:] {
		other, exists := s.sets[key]
		if !exists {
			continue
		}
		others = append(others, other)
	}
	return first.Diff(others...)
}

// SetExists checks if a set key exists.
func (s *Store) SetExists(key string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.sets[key]
	return exists
}
