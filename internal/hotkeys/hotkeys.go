package hotkeys

import (
	"container/heap"
	"sync"
	"time"
)

// Entry represents a single hot key with its access count.
type Entry struct {
	Key   string `json:"key"`
	Count int64  `json:"count"`
}

// Tracker tracks key access frequency and reports the top-N hottest keys.
// It is safe for concurrent use.
type Tracker struct {
	mu      sync.Mutex
	counts  map[string]*int64
	topN    int
	window  time.Duration // observation window
	started time.Time
}

// New creates a hot key tracker that retains counters for the top-N keys.
// The window parameter controls how often counters decay (0 = never decay).
func New(topN int, window time.Duration) *Tracker {
	if topN <= 0 {
		topN = 100
	}
	t := &Tracker{
		counts:  make(map[string]*int64, topN*2),
		topN:    topN,
		window:  window,
		started: time.Now(),
	}
	if window > 0 {
		go t.decayLoop()
	}
	return t
}

// Record records one access to the given key.
func (t *Tracker) Record(key string) {
	t.mu.Lock()
	if c, ok := t.counts[key]; ok {
		*c++
	} else {
		v := int64(1)
		t.counts[key] = &v
	}
	t.mu.Unlock()
}

// Top returns the top-N keys by access count, sorted descending.
func (t *Tracker) Top(n int) []Entry {
	if n <= 0 {
		n = t.topN
	}
	t.mu.Lock()
	defer t.mu.Unlock()

	h := &entryHeap{}
	heap.Init(h)

	for key, cnt := range t.counts {
		e := Entry{Key: key, Count: *cnt}
		if h.Len() < n {
			heap.Push(h, e)
		} else if (*h)[0].Count < e.Count {
			(*h)[0] = e
			heap.Fix(h, 0)
		}
	}

	result := make([]Entry, h.Len())
	for i := len(result) - 1; i >= 0; i-- {
		result[i] = heap.Pop(h).(Entry)
	}
	return result
}

// Reset clears all counters.
func (t *Tracker) Reset() {
	t.mu.Lock()
	t.counts = make(map[string]*int64, t.topN*2)
	t.started = time.Now()
	t.mu.Unlock()
}

// Size returns the number of tracked keys.
func (t *Tracker) Size() int {
	t.mu.Lock()
	defer t.mu.Unlock()
	return len(t.counts)
}

// decayLoop halves all counters every window period to ensure the tracker
// reflects recent access patterns rather than cumulative history.
func (t *Tracker) decayLoop() {
	ticker := time.NewTicker(t.window)
	defer ticker.Stop()

	for range ticker.C {
		t.mu.Lock()
		for key, cnt := range t.counts {
			*cnt /= 2
			if *cnt == 0 {
				delete(t.counts, key)
			}
		}
		t.mu.Unlock()
	}
}

// --- min-heap for top-N selection ---

type entryHeap []Entry

func (h entryHeap) Len() int            { return len(h) }
func (h entryHeap) Less(i, j int) bool  { return h[i].Count < h[j].Count }
func (h entryHeap) Swap(i, j int)       { h[i], h[j] = h[j], h[i] }
func (h *entryHeap) Push(x interface{}) { *h = append(*h, x.(Entry)) }

func (h *entryHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}
