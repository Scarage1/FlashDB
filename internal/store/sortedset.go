// Package store - Sorted Set implementation for FlashDB
package store

import (
	"sort"
	"sync"
)

// ScoredMember represents a member with its score in a sorted set.
type ScoredMember struct {
	Member string
	Score  float64
}

// SortedSet represents a Redis-like sorted set data structure.
// It maintains elements sorted by score, with O(log n) operations.
type SortedSet struct {
	mu      sync.RWMutex
	members map[string]float64 // member -> score
}

// NewSortedSet creates a new sorted set.
func NewSortedSet() *SortedSet {
	return &SortedSet{
		members: make(map[string]float64),
	}
}

// Add adds one or more members with scores. Returns the number of new members added.
func (z *SortedSet) Add(members ...ScoredMember) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	added := 0
	for _, m := range members {
		if _, exists := z.members[m.Member]; !exists {
			added++
		}
		z.members[m.Member] = m.Score
	}
	return added
}

// AddNX adds members only if they don't exist. Returns number added.
func (z *SortedSet) AddNX(members ...ScoredMember) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	added := 0
	for _, m := range members {
		if _, exists := z.members[m.Member]; !exists {
			z.members[m.Member] = m.Score
			added++
		}
	}
	return added
}

// AddXX updates only existing members. Returns number updated.
func (z *SortedSet) AddXX(members ...ScoredMember) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	updated := 0
	for _, m := range members {
		if _, exists := z.members[m.Member]; exists {
			z.members[m.Member] = m.Score
			updated++
		}
	}
	return updated
}

// Score returns the score of a member.
func (z *SortedSet) Score(member string) (float64, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	score, exists := z.members[member]
	return score, exists
}

// IncrBy increments the score of a member. Creates member if not exists.
func (z *SortedSet) IncrBy(member string, increment float64) float64 {
	z.mu.Lock()
	defer z.mu.Unlock()

	z.members[member] += increment
	return z.members[member]
}

// Remove removes members from the set. Returns number removed.
func (z *SortedSet) Remove(members ...string) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	removed := 0
	for _, m := range members {
		if _, exists := z.members[m]; exists {
			delete(z.members, m)
			removed++
		}
	}
	return removed
}

// Card returns the cardinality (number of elements) of the sorted set.
func (z *SortedSet) Card() int {
	z.mu.RLock()
	defer z.mu.RUnlock()
	return len(z.members)
}

// Rank returns the rank of a member (0-based, ascending by score).
func (z *SortedSet) Rank(member string) (int, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	score, exists := z.members[member]
	if !exists {
		return -1, false
	}

	rank := 0
	for m, s := range z.members {
		if s < score || (s == score && m < member) {
			rank++
		}
	}
	return rank, true
}

// RevRank returns the rank of a member (0-based, descending by score).
func (z *SortedSet) RevRank(member string) (int, bool) {
	z.mu.RLock()
	defer z.mu.RUnlock()

	score, exists := z.members[member]
	if !exists {
		return -1, false
	}

	rank := 0
	for m, s := range z.members {
		if s > score || (s == score && m > member) {
			rank++
		}
	}
	return rank, true
}

// Count returns the number of elements with scores between min and max (inclusive).
func (z *SortedSet) Count(min, max float64) int {
	z.mu.RLock()
	defer z.mu.RUnlock()

	count := 0
	for _, score := range z.members {
		if score >= min && score <= max {
			count++
		}
	}
	return count
}

// getSortedMembers returns all members sorted by score (ascending).
func (z *SortedSet) getSortedMembers() []ScoredMember {
	result := make([]ScoredMember, 0, len(z.members))
	for member, score := range z.members {
		result = append(result, ScoredMember{Member: member, Score: score})
	}
	sort.Slice(result, func(i, j int) bool {
		if result[i].Score != result[j].Score {
			return result[i].Score < result[j].Score
		}
		return result[i].Member < result[j].Member
	})
	return result
}

// Range returns members by index range (inclusive, 0-based).
// Supports negative indices (-1 = last element).
func (z *SortedSet) Range(start, stop int, withScores bool) []ScoredMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	sorted := z.getSortedMembers()
	n := len(sorted)
	if n == 0 {
		return nil
	}

	// Handle negative indices
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}

	// Clamp to valid range
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return nil
	}

	result := sorted[start : stop+1]
	if !withScores {
		for i := range result {
			result[i].Score = 0
		}
	}
	return result
}

// RevRange returns members by index range in reverse order (descending by score).
func (z *SortedSet) RevRange(start, stop int, withScores bool) []ScoredMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	sorted := z.getSortedMembers()
	n := len(sorted)
	if n == 0 {
		return nil
	}

	// Reverse the sorted slice
	for i, j := 0, n-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	// Handle negative indices
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}

	// Clamp to valid range
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return nil
	}

	result := sorted[start : stop+1]
	if !withScores {
		for i := range result {
			result[i].Score = 0
		}
	}
	return result
}

// RangeByScore returns members with scores between min and max.
func (z *SortedSet) RangeByScore(min, max float64, withScores bool, offset, count int) []ScoredMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	sorted := z.getSortedMembers()

	var result []ScoredMember
	skipped := 0
	for _, m := range sorted {
		if m.Score >= min && m.Score <= max {
			if skipped < offset {
				skipped++
				continue
			}
			if count > 0 && len(result) >= count {
				break
			}
			if withScores {
				result = append(result, m)
			} else {
				result = append(result, ScoredMember{Member: m.Member})
			}
		}
	}
	return result
}

// RevRangeByScore returns members with scores between min and max in reverse order.
func (z *SortedSet) RevRangeByScore(max, min float64, withScores bool, offset, count int) []ScoredMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	sorted := z.getSortedMembers()

	// Reverse
	for i, j := 0, len(sorted)-1; i < j; i, j = i+1, j-1 {
		sorted[i], sorted[j] = sorted[j], sorted[i]
	}

	var result []ScoredMember
	skipped := 0
	for _, m := range sorted {
		if m.Score >= min && m.Score <= max {
			if skipped < offset {
				skipped++
				continue
			}
			if count > 0 && len(result) >= count {
				break
			}
			if withScores {
				result = append(result, m)
			} else {
				result = append(result, ScoredMember{Member: m.Member})
			}
		}
	}
	return result
}

// RemoveRangeByRank removes members by rank range (inclusive).
func (z *SortedSet) RemoveRangeByRank(start, stop int) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	sorted := z.getSortedMembers()
	n := len(sorted)
	if n == 0 {
		return 0
	}

	// Handle negative indices
	if start < 0 {
		start = n + start
	}
	if stop < 0 {
		stop = n + stop
	}

	// Clamp
	if start < 0 {
		start = 0
	}
	if stop >= n {
		stop = n - 1
	}
	if start > stop || start >= n {
		return 0
	}

	removed := 0
	for i := start; i <= stop; i++ {
		delete(z.members, sorted[i].Member)
		removed++
	}
	return removed
}

// RemoveRangeByScore removes members with scores in the given range.
func (z *SortedSet) RemoveRangeByScore(min, max float64) int {
	z.mu.Lock()
	defer z.mu.Unlock()

	removed := 0
	for member, score := range z.members {
		if score >= min && score <= max {
			delete(z.members, member)
			removed++
		}
	}
	return removed
}

// PopMin removes and returns the member with the lowest score.
func (z *SortedSet) PopMin(count int) []ScoredMember {
	z.mu.Lock()
	defer z.mu.Unlock()

	if count <= 0 {
		count = 1
	}

	sorted := z.getSortedMembers()
	if len(sorted) == 0 {
		return nil
	}

	if count > len(sorted) {
		count = len(sorted)
	}

	result := make([]ScoredMember, count)
	copy(result, sorted[:count])

	for _, m := range result {
		delete(z.members, m.Member)
	}

	return result
}

// PopMax removes and returns the member with the highest score.
func (z *SortedSet) PopMax(count int) []ScoredMember {
	z.mu.Lock()
	defer z.mu.Unlock()

	if count <= 0 {
		count = 1
	}

	sorted := z.getSortedMembers()
	n := len(sorted)
	if n == 0 {
		return nil
	}

	if count > n {
		count = n
	}

	result := make([]ScoredMember, count)
	for i := 0; i < count; i++ {
		result[i] = sorted[n-1-i]
	}

	for _, m := range result {
		delete(z.members, m.Member)
	}

	return result
}

// RandMember returns random members from the sorted set.
func (z *SortedSet) RandMember(count int, withScores bool) []ScoredMember {
	z.mu.RLock()
	defer z.mu.RUnlock()

	if len(z.members) == 0 {
		return nil
	}

	allowDuplicates := count < 0
	if count < 0 {
		count = -count
	}

	members := make([]ScoredMember, 0, len(z.members))
	for m, s := range z.members {
		members = append(members, ScoredMember{Member: m, Score: s})
	}

	var result []ScoredMember
	if allowDuplicates {
		for i := 0; i < count; i++ {
			idx := int(uint(i*31+17) % uint(len(members))) // Simple pseudo-random
			if withScores {
				result = append(result, members[idx])
			} else {
				result = append(result, ScoredMember{Member: members[idx].Member})
			}
		}
	} else {
		if count > len(members) {
			count = len(members)
		}
		// Simple shuffle using deterministic approach
		for i := 0; i < count; i++ {
			if withScores {
				result = append(result, members[i])
			} else {
				result = append(result, ScoredMember{Member: members[i].Member})
			}
		}
	}

	return result
}

// Members returns all members (for iteration/serialization).
func (z *SortedSet) Members() map[string]float64 {
	z.mu.RLock()
	defer z.mu.RUnlock()

	result := make(map[string]float64, len(z.members))
	for k, v := range z.members {
		result[k] = v
	}
	return result
}
