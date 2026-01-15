package store

import (
	"testing"
)

func TestSortedSet_Add(t *testing.T) {
	z := NewSortedSet()

	// Test adding new members
	added := z.Add(
		ScoredMember{Member: "alice", Score: 100},
		ScoredMember{Member: "bob", Score: 200},
		ScoredMember{Member: "charlie", Score: 150},
	)

	if added != 3 {
		t.Errorf("expected 3 new members, got %d", added)
	}

	if z.Card() != 3 {
		t.Errorf("expected cardinality 3, got %d", z.Card())
	}

	// Test updating existing member (should not count as new)
	added = z.Add(ScoredMember{Member: "alice", Score: 110})
	if added != 0 {
		t.Errorf("expected 0 new members for update, got %d", added)
	}

	// Verify score was updated
	score, _ := z.Score("alice")
	if score != 110 {
		t.Errorf("expected score 110, got %f", score)
	}
}

func TestSortedSet_Score(t *testing.T) {
	z := NewSortedSet()
	z.Add(ScoredMember{Member: "player1", Score: 500})

	score, exists := z.Score("player1")
	if !exists {
		t.Error("expected member to exist")
	}
	if score != 500 {
		t.Errorf("expected score 500, got %f", score)
	}

	_, exists = z.Score("nonexistent")
	if exists {
		t.Error("expected nonexistent member to not exist")
	}
}

func TestSortedSet_Remove(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
	)

	removed := z.Remove("a", "c")
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	if z.Card() != 1 {
		t.Errorf("expected 1 member remaining, got %d", z.Card())
	}

	// Remove non-existent
	removed = z.Remove("nonexistent")
	if removed != 0 {
		t.Errorf("expected 0 removed for nonexistent, got %d", removed)
	}
}

func TestSortedSet_Rank(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "alice", Score: 100},
		ScoredMember{Member: "bob", Score: 200},
		ScoredMember{Member: "charlie", Score: 150},
	)

	// Ascending order: alice(100) < charlie(150) < bob(200)
	rank, exists := z.Rank("alice")
	if !exists || rank != 0 {
		t.Errorf("expected alice rank 0, got %d", rank)
	}

	rank, exists = z.Rank("charlie")
	if !exists || rank != 1 {
		t.Errorf("expected charlie rank 1, got %d", rank)
	}

	rank, exists = z.Rank("bob")
	if !exists || rank != 2 {
		t.Errorf("expected bob rank 2, got %d", rank)
	}

	_, exists = z.Rank("nonexistent")
	if exists {
		t.Error("expected nonexistent to have no rank")
	}
}

func TestSortedSet_RevRank(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "alice", Score: 100},
		ScoredMember{Member: "bob", Score: 200},
		ScoredMember{Member: "charlie", Score: 150},
	)

	// Descending order: bob(200) > charlie(150) > alice(100)
	rank, exists := z.RevRank("bob")
	if !exists || rank != 0 {
		t.Errorf("expected bob revrank 0, got %d", rank)
	}

	rank, exists = z.RevRank("charlie")
	if !exists || rank != 1 {
		t.Errorf("expected charlie revrank 1, got %d", rank)
	}

	rank, exists = z.RevRank("alice")
	if !exists || rank != 2 {
		t.Errorf("expected alice revrank 2, got %d", rank)
	}
}

func TestSortedSet_Range(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
		ScoredMember{Member: "d", Score: 4},
		ScoredMember{Member: "e", Score: 5},
	)

	// Get first 3
	members := z.Range(0, 2, false)
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}
	if members[0].Member != "a" || members[1].Member != "b" || members[2].Member != "c" {
		t.Error("unexpected member order")
	}

	// Test with negative indices
	members = z.Range(-2, -1, false)
	if len(members) != 2 {
		t.Errorf("expected 2 members, got %d", len(members))
	}
	if members[0].Member != "d" || members[1].Member != "e" {
		t.Error("unexpected members for negative indices")
	}

	// Test withScores
	members = z.Range(0, 0, true)
	if len(members) != 1 || members[0].Score != 1 {
		t.Error("expected score to be included")
	}
}

func TestSortedSet_RevRange(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
	)

	members := z.RevRange(0, 2, false)
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}
	if members[0].Member != "c" || members[1].Member != "b" || members[2].Member != "a" {
		t.Error("unexpected reverse order")
	}
}

func TestSortedSet_RangeByScore(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 10},
		ScoredMember{Member: "b", Score: 20},
		ScoredMember{Member: "c", Score: 30},
		ScoredMember{Member: "d", Score: 40},
		ScoredMember{Member: "e", Score: 50},
	)

	// Get members with score 20-40
	members := z.RangeByScore(20, 40, false, 0, -1)
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}

	// Test with offset and count
	members = z.RangeByScore(10, 50, false, 1, 2)
	if len(members) != 2 {
		t.Errorf("expected 2 members with limit, got %d", len(members))
	}
	if members[0].Member != "b" {
		t.Error("expected first member to be 'b' after offset")
	}
}

func TestSortedSet_Count(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 10},
		ScoredMember{Member: "b", Score: 20},
		ScoredMember{Member: "c", Score: 30},
	)

	count := z.Count(15, 35)
	if count != 2 {
		t.Errorf("expected count 2, got %d", count)
	}

	count = z.Count(0, 100)
	if count != 3 {
		t.Errorf("expected count 3, got %d", count)
	}

	count = z.Count(100, 200)
	if count != 0 {
		t.Errorf("expected count 0, got %d", count)
	}
}

func TestSortedSet_IncrBy(t *testing.T) {
	z := NewSortedSet()
	z.Add(ScoredMember{Member: "player", Score: 100})

	newScore := z.IncrBy("player", 50)
	if newScore != 150 {
		t.Errorf("expected new score 150, got %f", newScore)
	}

	// IncrBy on non-existent member should create it
	newScore = z.IncrBy("newplayer", 25)
	if newScore != 25 {
		t.Errorf("expected new score 25, got %f", newScore)
	}
}

func TestSortedSet_PopMin(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
	)

	popped := z.PopMin(2)
	if len(popped) != 2 {
		t.Errorf("expected 2 popped, got %d", len(popped))
	}
	if popped[0].Member != "a" || popped[1].Member != "b" {
		t.Error("expected lowest scoring members")
	}

	if z.Card() != 1 {
		t.Errorf("expected 1 member remaining, got %d", z.Card())
	}
}

func TestSortedSet_PopMax(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
	)

	popped := z.PopMax(2)
	if len(popped) != 2 {
		t.Errorf("expected 2 popped, got %d", len(popped))
	}
	if popped[0].Member != "c" || popped[1].Member != "b" {
		t.Error("expected highest scoring members")
	}

	if z.Card() != 1 {
		t.Errorf("expected 1 member remaining, got %d", z.Card())
	}
}

func TestSortedSet_RemoveRangeByRank(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
		ScoredMember{Member: "d", Score: 4},
	)

	removed := z.RemoveRangeByRank(1, 2)
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	if z.Card() != 2 {
		t.Errorf("expected 2 remaining, got %d", z.Card())
	}

	// Should have a and d remaining
	_, exists := z.Score("a")
	if !exists {
		t.Error("expected 'a' to exist")
	}
	_, exists = z.Score("d")
	if !exists {
		t.Error("expected 'd' to exist")
	}
}

func TestSortedSet_RemoveRangeByScore(t *testing.T) {
	z := NewSortedSet()
	z.Add(
		ScoredMember{Member: "a", Score: 10},
		ScoredMember{Member: "b", Score: 20},
		ScoredMember{Member: "c", Score: 30},
		ScoredMember{Member: "d", Score: 40},
	)

	removed := z.RemoveRangeByScore(15, 35)
	if removed != 2 {
		t.Errorf("expected 2 removed, got %d", removed)
	}

	if z.Card() != 2 {
		t.Errorf("expected 2 remaining, got %d", z.Card())
	}
}

func TestStore_ZAdd(t *testing.T) {
	s := New()
	defer s.Close()

	added := s.ZAdd("leaderboard",
		ScoredMember{Member: "player1", Score: 100},
		ScoredMember{Member: "player2", Score: 200},
	)

	if added != 2 {
		t.Errorf("expected 2 added, got %d", added)
	}

	card := s.ZCard("leaderboard")
	if card != 2 {
		t.Errorf("expected cardinality 2, got %d", card)
	}
}

func TestStore_ZRange(t *testing.T) {
	s := New()
	defer s.Close()

	s.ZAdd("scores",
		ScoredMember{Member: "a", Score: 1},
		ScoredMember{Member: "b", Score: 2},
		ScoredMember{Member: "c", Score: 3},
	)

	members := s.ZRange("scores", 0, -1, true)
	if len(members) != 3 {
		t.Errorf("expected 3 members, got %d", len(members))
	}

	// Non-existent key
	members = s.ZRange("nonexistent", 0, -1, false)
	if members != nil {
		t.Error("expected nil for non-existent key")
	}
}

func TestStore_Clear_SortedSets(t *testing.T) {
	s := New()
	defer s.Close()

	s.ZAdd("zset1", ScoredMember{Member: "m1", Score: 1})
	s.ZAdd("zset2", ScoredMember{Member: "m2", Score: 2})

	s.Clear()

	if s.ZCard("zset1") != 0 || s.ZCard("zset2") != 0 {
		t.Error("expected sorted sets to be cleared")
	}
}
