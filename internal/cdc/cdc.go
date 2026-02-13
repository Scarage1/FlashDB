package cdc

import (
	"encoding/json"
	"sync"
	"sync/atomic"
	"time"
)

// OpType describes the kind of mutation.
type OpType string

const (
	OpSet    OpType = "SET"
	OpDel    OpType = "DEL"
	OpExpire OpType = "EXPIRE"
	OpHSet   OpType = "HSET"
	OpHDel   OpType = "HDEL"
	OpLPush  OpType = "LPUSH"
	OpRPush  OpType = "RPUSH"
	OpLPop   OpType = "LPOP"
	OpRPop   OpType = "RPOP"
	OpSAdd   OpType = "SADD"
	OpSRem   OpType = "SREM"
	OpZAdd   OpType = "ZADD"
	OpZRem   OpType = "ZREM"
	OpTSAdd  OpType = "TS.ADD"
)

// Event represents a single mutation captured by CDC.
type Event struct {
	ID        uint64 `json:"id"`
	Timestamp int64  `json:"ts"`
	Op        OpType `json:"op"`
	Key       string `json:"key"`
	Value     string `json:"value,omitempty"`
	Field     string `json:"field,omitempty"`
}

// JSON returns the JSON encoding of the event.
func (e *Event) JSON() []byte {
	b, _ := json.Marshal(e)
	return b
}

// Stream is a thread-safe ring buffer of CDC events with subscriber support.
type Stream struct {
	mu      sync.RWMutex
	buf     []Event
	head    int
	size    int
	cap     int
	seq     atomic.Uint64
	subs    map[uint64]chan Event
	subMu   sync.Mutex
	nextSub uint64
}

// NewStream creates a CDC stream with the given ring buffer capacity.
func NewStream(capacity int) *Stream {
	if capacity <= 0 {
		capacity = 10000
	}
	return &Stream{
		buf:  make([]Event, capacity),
		cap:  capacity,
		subs: make(map[uint64]chan Event),
	}
}

// Record appends a new event to the stream and notifies all subscribers.
func (s *Stream) Record(op OpType, key, value, field string) Event {
	id := s.seq.Add(1)

	ev := Event{
		ID:        id,
		Timestamp: time.Now().UnixMilli(),
		Op:        op,
		Key:       key,
		Value:     value,
		Field:     field,
	}

	s.mu.Lock()
	s.buf[s.head] = ev
	s.head = (s.head + 1) % s.cap
	if s.size < s.cap {
		s.size++
	}
	s.mu.Unlock()

	// Notify subscribers (non-blocking)
	s.subMu.Lock()
	for _, ch := range s.subs {
		select {
		case ch <- ev:
		default:
		}
	}
	s.subMu.Unlock()

	return ev
}

// Since returns all events with ID > afterID, in order.
func (s *Stream) Since(afterID uint64) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []Event

	if s.size == 0 {
		return result
	}

	start := s.head - s.size
	if start < 0 {
		start += s.cap
	}

	for i := 0; i < s.size; i++ {
		idx := (start + i) % s.cap
		if s.buf[idx].ID > afterID {
			result = append(result, s.buf[idx])
		}
	}
	return result
}

// Latest returns the N most recent events.
func (s *Stream) Latest(n int) []Event {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if n > s.size {
		n = s.size
	}
	if n <= 0 {
		return nil
	}

	result := make([]Event, n)
	for i := 0; i < n; i++ {
		idx := s.head - n + i
		if idx < 0 {
			idx += s.cap
		}
		result[i] = s.buf[idx]
	}
	return result
}

// Subscribe creates a channel that receives real-time events.
func (s *Stream) Subscribe(bufSize int) (uint64, <-chan Event) {
	if bufSize <= 0 {
		bufSize = 256
	}
	ch := make(chan Event, bufSize)

	s.subMu.Lock()
	s.nextSub++
	id := s.nextSub
	s.subs[id] = ch
	s.subMu.Unlock()

	return id, ch
}

// Unsubscribe removes a subscriber and closes its channel.
func (s *Stream) Unsubscribe(id uint64) {
	s.subMu.Lock()
	if ch, ok := s.subs[id]; ok {
		close(ch)
		delete(s.subs, id)
	}
	s.subMu.Unlock()
}

// Stats returns current stream statistics.
type Stats struct {
	TotalEvents uint64 `json:"total_events"`
	BufferSize  int    `json:"buffer_size"`
	BufferCap   int    `json:"buffer_cap"`
	Subscribers int    `json:"subscribers"`
}

func (s *Stream) Stats() Stats {
	s.mu.RLock()
	size := s.size
	s.mu.RUnlock()

	s.subMu.Lock()
	subs := len(s.subs)
	s.subMu.Unlock()

	return Stats{
		TotalEvents: s.seq.Load(),
		BufferSize:  size,
		BufferCap:   s.cap,
		Subscribers: subs,
	}
}
