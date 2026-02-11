// Package engine provides the storage engine that coordinates WAL and in-memory store.
// All write operations follow the pattern: WAL append -> apply -> respond.
package engine

import (
	"fmt"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flashdb/flashdb/internal/store"
	"github.com/flashdb/flashdb/internal/wal"
)

// Stats holds engine statistics.
type Stats struct {
	TotalCommands int64
	TotalReads    int64
	TotalWrites   int64
	StartTime     time.Time
	KeysCount     int
	ExpiredKeys   int64
}

// Engine coordinates the WAL and in-memory store for durable key-value storage.
// It is safe for concurrent use by multiple goroutines.
type Engine struct {
	mu    sync.RWMutex
	store *store.Store
	wal   *wal.WAL

	startTime     time.Time
	totalCommands atomic.Int64
	totalReads    atomic.Int64
	totalWrites   atomic.Int64
	expiredKeys   atomic.Int64
}

// New creates a new Engine with the specified WAL path.
// It recovers any existing data from the WAL on startup.
func New(walPath string) (*Engine, error) {
	w, err := wal.Open(walPath)
	if err != nil {
		return nil, fmt.Errorf("engine: failed to open WAL: %w", err)
	}

	s := store.New()
	e := &Engine{
		store:     s,
		wal:       w,
		startTime: time.Now(),
	}

	// Recover from WAL
	if err := e.recover(); err != nil {
		w.Close()
		s.Close()
		return nil, fmt.Errorf("engine: failed to recover: %w", err)
	}

	return e, nil
}

// recover replays the WAL to restore state.
func (e *Engine) recover() error {
	records, err := e.wal.ReadAll()
	if err != nil {
		return err
	}

	for _, rec := range records {
		switch rec.Type {
		case wal.OpSet:
			e.store.Set(string(rec.Key), rec.Value)
		case wal.OpSetWithTTL:
			if rec.ExpireAt > 0 {
				expireTime := time.UnixMilli(rec.ExpireAt)
				if time.Now().Before(expireTime) {
					entry := &store.Entry{
						Value:     rec.Value,
						ExpireAt:  expireTime,
						HasExpire: true,
					}
					e.store.SetEntry(string(rec.Key), entry)
				}
				// Skip expired keys during recovery
			} else {
				e.store.Set(string(rec.Key), rec.Value)
			}
		case wal.OpDelete:
			e.store.Delete(string(rec.Key))
		case wal.OpExpire:
			if rec.ExpireAt > 0 {
				expireTime := time.UnixMilli(rec.ExpireAt)
				ttl := time.Until(expireTime)
				if ttl > 0 {
					e.store.Expire(string(rec.Key), ttl)
				}
			}
		case wal.OpPersist:
			e.store.Persist(string(rec.Key))
		}
	}

	return nil
}

func (e *Engine) recordRead() {
	e.totalReads.Add(1)
	e.totalCommands.Add(1)
}

func (e *Engine) recordWrite() {
	e.totalWrites.Add(1)
	e.totalCommands.Add(1)
}

func (e *Engine) recordCommand() {
	e.totalCommands.Add(1)
}

// Set stores a key-value pair.
// The operation is persisted to WAL before being applied.
func (e *Engine) Set(key string, value []byte) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	rec := wal.Record{
		Type:  wal.OpSet,
		Key:   []byte(key),
		Value: value,
	}
	if err := e.wal.Append(rec); err != nil {
		return fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.store.Set(key, value)
	e.recordWrite()
	return nil
}

// SetWithTTL stores a key-value pair with expiration.
func (e *Engine) SetWithTTL(key string, value []byte, ttl time.Duration) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	expireAt := time.Now().Add(ttl).UnixMilli()
	rec := wal.Record{
		Type:     wal.OpSetWithTTL,
		Key:      []byte(key),
		Value:    value,
		ExpireAt: expireAt,
	}
	if err := e.wal.Append(rec); err != nil {
		return fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.store.SetWithTTL(key, value, ttl)
	e.recordWrite()
	return nil
}

// SetNX sets key if it doesn't exist. Returns true if set.
func (e *Engine) SetNX(key string, value []byte) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if e.store.Exists(key) {
		e.recordCommand()
		return false, nil
	}

	rec := wal.Record{
		Type:  wal.OpSet,
		Key:   []byte(key),
		Value: value,
	}
	if err := e.wal.Append(rec); err != nil {
		return false, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.store.Set(key, value)
	e.recordWrite()
	return true, nil
}

// Get retrieves a value by key.
// Returns the value and true if found, nil and false otherwise.
func (e *Engine) Get(key string) ([]byte, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()
	return e.store.Get(key)
}

// Delete removes a key from the store.
// Returns true if the key existed, false otherwise.
func (e *Engine) Delete(key string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	exists := e.store.Exists(key)
	if !exists {
		e.recordCommand()
		return false, nil
	}

	rec := wal.Record{
		Type:  wal.OpDelete,
		Key:   []byte(key),
		Value: nil,
	}
	if err := e.wal.Append(rec); err != nil {
		return false, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.store.Delete(key)
	e.recordWrite()
	return true, nil
}

// Exists checks if a key exists in the store.
func (e *Engine) Exists(key string) bool {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()
	return e.store.Exists(key)
}

// Expire sets TTL on an existing key.
func (e *Engine) Expire(key string, ttl time.Duration) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	if !e.store.Exists(key) {
		e.recordCommand()
		return false, nil
	}

	expireAt := time.Now().Add(ttl).UnixMilli()
	rec := wal.Record{
		Type:     wal.OpExpire,
		Key:      []byte(key),
		ExpireAt: expireAt,
	}
	if err := e.wal.Append(rec); err != nil {
		return false, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.store.Expire(key, ttl)
	e.recordWrite()
	return true, nil
}

// TTL returns the remaining TTL for a key in seconds.
// Returns -2 if key doesn't exist, -1 if no TTL.
func (e *Engine) TTL(key string) int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()

	ttl := e.store.TTL(key)
	return int64(ttl.Seconds())
}

// PTTL returns the remaining TTL for a key in milliseconds.
func (e *Engine) PTTL(key string) int64 {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()

	ttl := e.store.TTL(key)
	return ttl.Milliseconds()
}

// Persist removes TTL from a key.
func (e *Engine) Persist(key string) (bool, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	entry, exists := e.store.GetEntry(key)
	if !exists || !entry.HasExpire {
		e.recordCommand()
		return false, nil
	}

	rec := wal.Record{
		Type: wal.OpPersist,
		Key:  []byte(key),
	}
	if err := e.wal.Append(rec); err != nil {
		return false, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	persisted := e.store.Persist(key)
	if persisted {
		e.recordWrite()
		return true, nil
	}

	e.recordCommand()
	return false, nil
}

// Keys returns all keys in the store.
func (e *Engine) Keys() []string {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()
	return e.store.Keys()
}

// Size returns the number of keys in the store.
func (e *Engine) Size() int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()
	return e.store.Size()
}

// Append appends value to key and returns new length.
func (e *Engine) Append(key string, value []byte) (int, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Get current value
	current, _ := e.store.Get(key)
	newValue := append(current, value...)

	rec := wal.Record{
		Type:  wal.OpSet,
		Key:   []byte(key),
		Value: newValue,
	}
	if err := e.wal.Append(rec); err != nil {
		return 0, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	length := e.store.Append(key, value)
	e.recordWrite()
	return length, nil
}

// StrLen returns string length of value at key.
func (e *Engine) StrLen(key string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()
	e.recordRead()
	return e.store.StrLen(key)
}

// IncrBy increments integer value by delta.
func (e *Engine) IncrBy(key string, delta int64) (int64, error) {
	e.mu.Lock()
	defer e.mu.Unlock()

	newVal, err := e.store.IncrBy(key, delta)
	if err != nil {
		e.recordCommand()
		return 0, err
	}

	rec := wal.Record{
		Type:  wal.OpSet,
		Key:   []byte(key),
		Value: []byte(fmt.Sprintf("%d", newVal)),
	}
	if err := e.wal.Append(rec); err != nil {
		return 0, fmt.Errorf("engine: failed to write WAL: %w", err)
	}

	e.recordWrite()
	return newVal, nil
}

// Clear removes all keys from the store and clears the WAL.
func (e *Engine) Clear() error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if err := e.wal.Clear(); err != nil {
		return fmt.Errorf("engine: failed to clear WAL: %w", err)
	}

	e.store.Clear()
	e.recordWrite()
	return nil
}

// GetStats returns engine statistics.
func (e *Engine) GetStats() Stats {
	e.mu.RLock()
	defer e.mu.RUnlock()
	return Stats{
		TotalCommands: e.totalCommands.Load(),
		TotalReads:    e.totalReads.Load(),
		TotalWrites:   e.totalWrites.Load(),
		StartTime:     e.startTime,
		KeysCount:     e.store.Size(),
		ExpiredKeys:   e.expiredKeys.Load(),
	}
}

// Close closes the engine and its underlying WAL.
func (e *Engine) Close() error {
	e.mu.Lock()
	defer e.mu.Unlock()
	e.store.Close()
	return e.wal.Close()
}

// ========================
// Sorted Set Operations
// ========================

// ZAdd adds members to a sorted set. Returns number of NEW members added.
func (e *Engine) ZAdd(key string, members ...store.ScoredMember) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	// Note: Sorted sets are currently in-memory only (not persisted to WAL)
	// For persistence, we'd need to add new WAL op types
	result := e.store.ZAdd(key, members...)
	e.recordWrite()
	return result
}

// ZScore returns the score of a member in a sorted set.
func (e *Engine) ZScore(key, member string) (float64, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZScore(key, member)
}

// ZRem removes members from a sorted set. Returns number removed.
func (e *Engine) ZRem(key string, members ...string) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZRem(key, members...)
	e.recordWrite()
	return result
}

// ZCard returns the cardinality of a sorted set.
func (e *Engine) ZCard(key string) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZCard(key)
}

// ZRank returns the rank of a member (0-based, ascending).
func (e *Engine) ZRank(key, member string) (int, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRank(key, member)
}

// ZRevRank returns the rank of a member (0-based, descending).
func (e *Engine) ZRevRank(key, member string) (int, bool) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRevRank(key, member)
}

// ZRange returns members by rank range.
func (e *Engine) ZRange(key string, start, stop int, withScores bool) []store.ScoredMember {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRange(key, start, stop, withScores)
}

// ZRevRange returns members by rank range in reverse order.
func (e *Engine) ZRevRange(key string, start, stop int, withScores bool) []store.ScoredMember {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRevRange(key, start, stop, withScores)
}

// ZRangeByScore returns members with scores in range.
func (e *Engine) ZRangeByScore(key string, min, max float64, withScores bool, offset, count int) []store.ScoredMember {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRangeByScore(key, min, max, withScores, offset, count)
}

// ZRevRangeByScore returns members with scores in range, descending.
func (e *Engine) ZRevRangeByScore(key string, max, min float64, withScores bool, offset, count int) []store.ScoredMember {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZRevRangeByScore(key, max, min, withScores, offset, count)
}

// ZCount returns count of members with scores in range.
func (e *Engine) ZCount(key string, min, max float64) int {
	e.mu.RLock()
	defer e.mu.RUnlock()

	e.recordRead()
	return e.store.ZCount(key, min, max)
}

// ZIncrBy increments score of member. Returns new score.
func (e *Engine) ZIncrBy(key, member string, increment float64) float64 {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZIncrBy(key, member, increment)
	e.recordWrite()
	return result
}

// ZRemRangeByRank removes members by rank range.
func (e *Engine) ZRemRangeByRank(key string, start, stop int) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZRemRangeByRank(key, start, stop)
	e.recordWrite()
	return result
}

// ZRemRangeByScore removes members by score range.
func (e *Engine) ZRemRangeByScore(key string, min, max float64) int {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZRemRangeByScore(key, min, max)
	e.recordWrite()
	return result
}

// ZPopMin removes and returns lowest-scoring members.
func (e *Engine) ZPopMin(key string, count int) []store.ScoredMember {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZPopMin(key, count)
	e.recordWrite()
	return result
}

// ZPopMax removes and returns highest-scoring members.
func (e *Engine) ZPopMax(key string, count int) []store.ScoredMember {
	e.mu.Lock()
	defer e.mu.Unlock()

	result := e.store.ZPopMax(key, count)
	e.recordWrite()
	return result
}
