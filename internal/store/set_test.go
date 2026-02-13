package store

import (
	"sort"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestSet_AddAndCard(t *testing.T) {
	s := NewSet()
	n := s.Add("a", "b", "c")
	assert.Equal(t, 3, n)
	assert.Equal(t, 3, s.Card())

	// Add duplicates
	n = s.Add("a", "d")
	assert.Equal(t, 1, n) // only "d" is new
	assert.Equal(t, 4, s.Card())
}

func TestSet_Rem(t *testing.T) {
	s := NewSet()
	s.Add("a", "b", "c")

	n := s.Rem("a", "missing")
	assert.Equal(t, 1, n)
	assert.Equal(t, 2, s.Card())
	assert.False(t, s.IsMember("a"))
}

func TestSet_IsMember(t *testing.T) {
	s := NewSet()
	s.Add("x")
	assert.True(t, s.IsMember("x"))
	assert.False(t, s.IsMember("y"))
}

func TestSet_Members(t *testing.T) {
	s := NewSet()
	s.Add("a", "b", "c")

	members := s.Members()
	sort.Strings(members)
	assert.Equal(t, []string{"a", "b", "c"}, members)
}

func TestSet_RandMember(t *testing.T) {
	s := NewSet()
	s.Add("a", "b", "c", "d", "e")

	members := s.RandMember(3)
	assert.Len(t, members, 3)
	// Each should be a valid member
	all := s.Members()
	for _, m := range members {
		assert.Contains(t, all, m)
	}

	// Request more than exists
	members = s.RandMember(10)
	assert.Len(t, members, 5)

	// Empty set
	empty := NewSet()
	members = empty.RandMember(1)
	assert.Len(t, members, 0)
}

func TestSet_Pop(t *testing.T) {
	s := NewSet()
	s.Add("a", "b", "c")

	popped := s.Pop(2)
	assert.Len(t, popped, 2)
	assert.Equal(t, 1, s.Card())

	// Pop remaining
	popped = s.Pop(5)
	assert.Len(t, popped, 1)
	assert.Equal(t, 0, s.Card())
}

func TestSet_Inter(t *testing.T) {
	s1 := NewSet()
	s1.Add("a", "b", "c")
	s2 := NewSet()
	s2.Add("b", "c", "d")
	s3 := NewSet()
	s3.Add("c", "d", "e")

	result := s1.Inter(s2, s3)
	assert.Equal(t, []string{"c"}, result)

	// No others
	result = s1.Inter()
	sort.Strings(result)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSet_Union(t *testing.T) {
	s1 := NewSet()
	s1.Add("a", "b")
	s2 := NewSet()
	s2.Add("b", "c")

	result := s1.Union(s2)
	sort.Strings(result)
	assert.Equal(t, []string{"a", "b", "c"}, result)
}

func TestSet_Diff(t *testing.T) {
	s1 := NewSet()
	s1.Add("a", "b", "c")
	s2 := NewSet()
	s2.Add("b", "d")

	result := s1.Diff(s2)
	sort.Strings(result)
	assert.Equal(t, []string{"a", "c"}, result)
}
