package timeseries

import (
	"fmt"
	"sort"
	"sync"
	"time"
)

// DataPoint is a single timestamped value in a time series.
type DataPoint struct {
	Timestamp int64   `json:"ts"`
	Value     float64 `json:"val"`
}

// Series holds a time-ordered sequence of data points with optional retention.
type Series struct {
	Points    []DataPoint
	Retention time.Duration // 0 = infinite
	Labels    map[string]string
}

// Store manages all time-series keys. It is safe for concurrent use.
type Store struct {
	mu     sync.RWMutex
	series map[string]*Series
	stopGC chan struct{}
}

// New creates a new time-series store and starts background retention cleanup.
func New() *Store {
	s := &Store{
		series: make(map[string]*Series),
		stopGC: make(chan struct{}),
	}
	go s.gcLoop()
	return s
}

// Close stops the background GC goroutine.
func (s *Store) Close() {
	close(s.stopGC)
}

// Add appends a data point to the series identified by key.
// If the series does not exist, it is created with the given retention.
// A timestamp of 0 means "now". Returns the inserted timestamp.
func (s *Store) Add(key string, ts int64, value float64, retention time.Duration) int64 {
	if ts <= 0 {
		ts = time.Now().UnixMilli()
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	ser, ok := s.series[key]
	if !ok {
		ser = &Series{
			Points:    make([]DataPoint, 0, 64),
			Retention: retention,
			Labels:    make(map[string]string),
		}
		s.series[key] = ser
	}

	dp := DataPoint{Timestamp: ts, Value: value}

	// Fast path: append if timestamp is >= last point (most common case)
	if len(ser.Points) == 0 || ts >= ser.Points[len(ser.Points)-1].Timestamp {
		ser.Points = append(ser.Points, dp)
	} else {
		// Insert in sorted position (out-of-order insert)
		idx := sort.Search(len(ser.Points), func(i int) bool {
			return ser.Points[i].Timestamp >= ts
		})
		ser.Points = append(ser.Points, DataPoint{})
		copy(ser.Points[idx+1:], ser.Points[idx:])
		ser.Points[idx] = dp
	}

	return ts
}

// Get returns the latest data point for the key, or false if it doesn't exist.
func (s *Store) Get(key string) (DataPoint, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ser, ok := s.series[key]
	if !ok || len(ser.Points) == 0 {
		return DataPoint{}, false
	}
	return ser.Points[len(ser.Points)-1], true
}

// Range returns data points within [fromTS, toTS] inclusive.
func (s *Store) Range(key string, fromTS, toTS int64) ([]DataPoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ser, ok := s.series[key]
	if !ok {
		return nil, fmt.Errorf("ERR no such key '%s'", key)
	}

	startIdx := sort.Search(len(ser.Points), func(i int) bool {
		return ser.Points[i].Timestamp >= fromTS
	})
	endIdx := sort.Search(len(ser.Points), func(i int) bool {
		return ser.Points[i].Timestamp > toTS
	})

	if startIdx >= endIdx {
		return []DataPoint{}, nil
	}

	result := make([]DataPoint, endIdx-startIdx)
	copy(result, ser.Points[startIdx:endIdx])
	return result, nil
}

// Info returns metadata about a time-series key.
type Info struct {
	TotalSamples int           `json:"total_samples"`
	FirstTS      int64         `json:"first_timestamp"`
	LastTS       int64         `json:"last_timestamp"`
	Retention    time.Duration `json:"retention_ms"`
	MemoryBytes  int           `json:"memory_bytes"`
}

// GetInfo returns metadata for a given series key.
func (s *Store) GetInfo(key string) (Info, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	ser, ok := s.series[key]
	if !ok {
		return Info{}, fmt.Errorf("ERR no such key '%s'", key)
	}

	info := Info{
		TotalSamples: len(ser.Points),
		Retention:    ser.Retention,
		MemoryBytes:  len(ser.Points) * 16,
	}
	if len(ser.Points) > 0 {
		info.FirstTS = ser.Points[0].Timestamp
		info.LastTS = ser.Points[len(ser.Points)-1].Timestamp
	}
	return info, nil
}

// Delete removes a time-series key entirely.
func (s *Store) Delete(key string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	_, ok := s.series[key]
	delete(s.series, key)
	return ok
}

// Keys returns all time-series key names.
func (s *Store) Keys() []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]string, 0, len(s.series))
	for k := range s.series {
		keys = append(keys, k)
	}
	return keys
}

// Size returns the number of series.
func (s *Store) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.series)
}

// gcLoop removes expired data points from all series with a retention policy.
func (s *Store) gcLoop() {
	ticker := time.NewTicker(5 * time.Second)
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

func (s *Store) removeExpired() {
	now := time.Now().UnixMilli()

	s.mu.Lock()
	defer s.mu.Unlock()

	for key, ser := range s.series {
		if ser.Retention <= 0 {
			continue
		}
		cutoff := now - ser.Retention.Milliseconds()
		idx := sort.Search(len(ser.Points), func(i int) bool {
			return ser.Points[i].Timestamp >= cutoff
		})
		if idx > 0 {
			ser.Points = ser.Points[idx:]
		}
		if len(ser.Points) == 0 {
			delete(s.series, key)
		}
	}
}
