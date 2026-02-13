package hotkeys

import (
	"sync"
	"testing"
	"time"
)

func TestTracker_RecordAndTop(t *testing.T) {
	tr := New(10, 0)
	for i := 0; i < 100; i++ {
		tr.Record("hot")
	}
	for i := 0; i < 50; i++ {
		tr.Record("warm")
	}
	tr.Record("cold")

	top := tr.Top(3)
	if len(top) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(top))
	}
	if top[0].Key != "hot" || top[0].Count != 100 {
		t.Fatalf("expected hot:100, got %s:%d", top[0].Key, top[0].Count)
	}
	if top[1].Key != "warm" || top[1].Count != 50 {
		t.Fatalf("expected warm:50, got %s:%d", top[1].Key, top[1].Count)
	}
}

func TestTracker_TopN_Limit(t *testing.T) {
	tr := New(2, 0)
	tr.Record("a")
	tr.Record("b")
	tr.Record("c")

	top := tr.Top(2)
	if len(top) != 2 {
		t.Fatalf("expected 2 entries, got %d", len(top))
	}
}

func TestTracker_Reset(t *testing.T) {
	tr := New(10, 0)
	tr.Record("x")
	tr.Reset()
	if tr.Size() != 0 {
		t.Fatalf("expected 0 after reset, got %d", tr.Size())
	}
}

func TestTracker_Decay(t *testing.T) {
	tr := New(10, 50*time.Millisecond)
	for i := 0; i < 100; i++ {
		tr.Record("key")
	}
	time.Sleep(120 * time.Millisecond) // wait for at least one decay

	top := tr.Top(1)
	if len(top) == 0 {
		return // fully decayed is acceptable
	}
	if top[0].Count >= 100 {
		t.Fatalf("expected count < 100 after decay, got %d", top[0].Count)
	}
}

func TestTracker_ConcurrentAccess(t *testing.T) {
	tr := New(10, 0)
	var wg sync.WaitGroup
	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 100; j++ {
				tr.Record("concurrent")
			}
		}()
	}
	wg.Wait()

	top := tr.Top(1)
	if top[0].Count != 1000 {
		t.Fatalf("expected 1000, got %d", top[0].Count)
	}
}
