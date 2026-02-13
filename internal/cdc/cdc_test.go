package cdc

import (
	"sync"
	"testing"
	"time"
)

func TestRecord_And_Latest(t *testing.T) {
	s := NewStream(100)

	s.Record(OpSet, "k1", "v1", "")
	s.Record(OpSet, "k2", "v2", "")
	s.Record(OpDel, "k1", "", "")

	events := s.Latest(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events, got %d", len(events))
	}
	if events[0].Op != OpSet || events[0].Key != "k1" {
		t.Fatalf("unexpected first event: %+v", events[0])
	}
	if events[2].Op != OpDel || events[2].Key != "k1" {
		t.Fatalf("unexpected last event: %+v", events[2])
	}
}

func TestSince(t *testing.T) {
	s := NewStream(100)

	s.Record(OpSet, "a", "1", "")
	s.Record(OpSet, "b", "2", "")
	s.Record(OpSet, "c", "3", "")

	events := s.Since(1)
	if len(events) != 2 {
		t.Fatalf("expected 2 events after id 1, got %d", len(events))
	}
	if events[0].Key != "b" || events[1].Key != "c" {
		t.Fatalf("unexpected events: %+v", events)
	}
}

func TestRingBuffer_Wrap(t *testing.T) {
	s := NewStream(3)

	for i := 0; i < 5; i++ {
		s.Record(OpSet, "k", "v", "")
	}

	events := s.Latest(10)
	if len(events) != 3 {
		t.Fatalf("expected 3 events in full buffer, got %d", len(events))
	}
	if events[0].ID != 3 {
		t.Fatalf("expected oldest event ID 3, got %d", events[0].ID)
	}
	if events[2].ID != 5 {
		t.Fatalf("expected newest event ID 5, got %d", events[2].ID)
	}
}

func TestSubscribe(t *testing.T) {
	s := NewStream(100)

	subID, ch := s.Subscribe(10)
	defer s.Unsubscribe(subID)

	s.Record(OpSet, "key", "val", "")

	select {
	case ev := <-ch:
		if ev.Key != "key" || ev.Op != OpSet {
			t.Fatalf("unexpected event: %+v", ev)
		}
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for event")
	}
}

func TestUnsubscribe(t *testing.T) {
	s := NewStream(100)

	subID, ch := s.Subscribe(10)
	s.Unsubscribe(subID)

	_, ok := <-ch
	if ok {
		t.Fatal("expected channel to be closed")
	}
}

func TestStats(t *testing.T) {
	s := NewStream(50)

	s.Record(OpSet, "a", "1", "")
	s.Record(OpDel, "a", "", "")
	subID, _ := s.Subscribe(10)
	defer s.Unsubscribe(subID)

	stats := s.Stats()
	if stats.TotalEvents != 2 {
		t.Fatalf("expected 2 total events, got %d", stats.TotalEvents)
	}
	if stats.BufferSize != 2 {
		t.Fatalf("expected buffer size 2, got %d", stats.BufferSize)
	}
	if stats.BufferCap != 50 {
		t.Fatalf("expected cap 50, got %d", stats.BufferCap)
	}
	if stats.Subscribers != 1 {
		t.Fatalf("expected 1 subscriber, got %d", stats.Subscribers)
	}
}

func TestConcurrentAccess(t *testing.T) {
	s := NewStream(1000)
	var wg sync.WaitGroup

	subID, ch := s.Subscribe(500)
	defer s.Unsubscribe(subID)

	wg.Add(1)
	go func() {
		defer wg.Done()
		for i := 0; i < 100; i++ {
			s.Record(OpSet, "k", "v", "")
		}
	}()

	consumed := 0
	wg.Add(1)
	go func() {
		defer wg.Done()
		timeout := time.After(2 * time.Second)
		for consumed < 100 {
			select {
			case <-ch:
				consumed++
			case <-timeout:
				return
			}
		}
	}()

	wg.Wait()
	if consumed != 100 {
		t.Fatalf("expected 100 consumed events, got %d", consumed)
	}
}

func TestEvent_JSON(t *testing.T) {
	ev := Event{
		ID:        1,
		Timestamp: 1234567890,
		Op:        OpHSet,
		Key:       "myhash",
		Value:     "val",
		Field:     "field1",
	}
	b := ev.JSON()
	if len(b) == 0 {
		t.Fatal("expected non-empty JSON")
	}
}
