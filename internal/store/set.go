// Package store - Set data type implementation for FlashDB
//
// A Set is an unordered collection of unique string members.
// Equivalent to Redis Sets. Add/Remove/IsMember operations are O(1).
// Set operations (inter/union/diff) are O(N*M) in the worst case.
package store

import "math/rand"

// Set represents a Redis-like set data structure.
// The Set itself is NOT thread-safe; concurrency is managed by the Store.
type Set struct {
	members map[string]struct{}
}

// NewSet creates a new empty Set.
func NewSet() *Set {
	return &Set{
		members: make(map[string]struct{}),
	}
}

// Add adds one or more members. Returns the number of members actually added (not already present).
func (s *Set) Add(members ...string) int {
	added := 0
	for _, m := range members {
		if _, exists := s.members[m]; !exists {
			s.members[m] = struct{}{}
			added++
		}
	}
	return added
}

// Rem removes one or more members. Returns the number of members actually removed.
func (s *Set) Rem(members ...string) int {
	removed := 0
	for _, m := range members {
		if _, exists := s.members[m]; exists {
			delete(s.members, m)
			removed++
		}
	}
	return removed
}

// IsMember returns true if the member exists in the set.
func (s *Set) IsMember(member string) bool {
	_, exists := s.members[member]
	return exists
}

// Card returns the number of members in the set.
func (s *Set) Card() int {
	return len(s.members)
}

// Members returns all members as a string slice.
func (s *Set) Members() []string {
	result := make([]string, 0, len(s.members))
	for m := range s.members {
		result = append(result, m)
	}
	return result
}

// RandMember returns count random members from the set.
//   - count > 0: Return up to count unique random members
//   - count < 0: Return |count| random members (may include duplicates)
//
// Returns nil if the set is empty.
func (s *Set) RandMember(count int) []string {
	if len(s.members) == 0 {
		return nil
	}

	members := s.Members()

	if count > 0 {
		// Unique random members
		if count > len(members) {
			count = len(members)
		}
		// Fisher-Yates shuffle
		rand.Shuffle(len(members), func(i, j int) {
			members[i], members[j] = members[j], members[i]
		})
		return members[:count]
	}

	// count < 0: allow duplicates
	absCount := -count
	result := make([]string, absCount)
	for i := 0; i < absCount; i++ {
		result[i] = members[rand.Intn(len(members))]
	}
	return result
}

// Pop removes and returns count random members.
func (s *Set) Pop(count int) []string {
	if len(s.members) == 0 || count <= 0 {
		return nil
	}

	members := s.Members()
	if count > len(members) {
		count = len(members)
	}

	rand.Shuffle(len(members), func(i, j int) {
		members[i], members[j] = members[j], members[i]
	})

	popped := members[:count]
	for _, m := range popped {
		delete(s.members, m)
	}
	return popped
}

// Inter returns the intersection of this set with the given sets.
func (s *Set) Inter(others ...*Set) []string {
	result := make([]string, 0)
	for member := range s.members {
		inAll := true
		for _, other := range others {
			if other == nil || !other.IsMember(member) {
				inAll = false
				break
			}
		}
		if inAll {
			result = append(result, member)
		}
	}
	return result
}

// Union returns the union of this set with the given sets.
func (s *Set) Union(others ...*Set) []string {
	seen := make(map[string]struct{})
	for m := range s.members {
		seen[m] = struct{}{}
	}
	for _, other := range others {
		if other == nil {
			continue
		}
		for m := range other.members {
			seen[m] = struct{}{}
		}
	}
	result := make([]string, 0, len(seen))
	for m := range seen {
		result = append(result, m)
	}
	return result
}

// Diff returns members in this set that are NOT in any of the given sets.
func (s *Set) Diff(others ...*Set) []string {
	result := make([]string, 0)
	for member := range s.members {
		inOther := false
		for _, other := range others {
			if other != nil && other.IsMember(member) {
				inOther = true
				break
			}
		}
		if !inOther {
			result = append(result, member)
		}
	}
	return result
}
