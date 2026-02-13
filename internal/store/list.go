// Package store - List data type implementation for FlashDB
//
// A List is a doubly-linked list of byte values stored under a single key.
// Equivalent to Redis Lists. Push/Pop operations are O(1).
// Index-based operations are O(N) where N is the distance to the target.
package store

import "fmt"

// List represents a Redis-like list data structure backed by a Go slice.
// The List itself is NOT thread-safe; concurrency is managed by the Store.
type List struct {
	items [][]byte
}

// NewList creates a new empty List.
func NewList() *List {
	return &List{
		items: make([][]byte, 0),
	}
}

// LPush prepends one or more values to the list.
// Values are inserted one at a time from left to right, so
// LPUSH mylist a b c will result in c b a (c is head).
// Returns the new length of the list.
func (l *List) LPush(values ...[]byte) int {
	// Each value is prepended, so we reverse to match Redis behavior:
	// LPUSH key a b c → c at head, then b at head, then a at head? No.
	// Actually Redis LPUSH key a b c pushes a first, then b before a, then c before b.
	// Result: c b a existing...
	newItems := make([][]byte, len(values)+len(l.items))
	for i, v := range values {
		newItems[len(values)-1-i] = cloneBytes(v)
	}
	copy(newItems[len(values):], l.items)
	l.items = newItems
	return len(l.items)
}

// RPush appends one or more values to the list.
// Returns the new length of the list.
func (l *List) RPush(values ...[]byte) int {
	for _, v := range values {
		l.items = append(l.items, cloneBytes(v))
	}
	return len(l.items)
}

// LPop removes and returns the first element.
func (l *List) LPop() ([]byte, bool) {
	if len(l.items) == 0 {
		return nil, false
	}
	val := l.items[0]
	l.items = l.items[1:]
	return val, true
}

// RPop removes and returns the last element.
func (l *List) RPop() ([]byte, bool) {
	if len(l.items) == 0 {
		return nil, false
	}
	val := l.items[len(l.items)-1]
	l.items = l.items[:len(l.items)-1]
	return val, true
}

// Len returns the number of elements in the list.
func (l *List) Len() int {
	return len(l.items)
}

// Index returns the element at the given index.
// Negative indices count from the end (-1 is the last element).
func (l *List) Index(index int) ([]byte, bool) {
	idx := l.resolveIndex(index)
	if idx < 0 || idx >= len(l.items) {
		return nil, false
	}
	return cloneBytes(l.items[idx]), true
}

// Set sets the element at index to value.
// Returns an error if the index is out of range.
func (l *List) Set(index int, value []byte) error {
	idx := l.resolveIndex(index)
	if idx < 0 || idx >= len(l.items) {
		return fmt.Errorf("index out of range")
	}
	l.items[idx] = cloneBytes(value)
	return nil
}

// Range returns elements from start to stop (inclusive), supporting negative indices.
func (l *List) Range(start, stop int) [][]byte {
	length := len(l.items)
	if length == 0 {
		return nil
	}

	// Resolve negative indices
	s := l.resolveIndex(start)
	e := l.resolveIndex(stop)

	// Clamp
	if s < 0 {
		s = 0
	}
	if e >= length {
		e = length - 1
	}
	if s > e {
		return nil
	}

	result := make([][]byte, e-s+1)
	for i := s; i <= e; i++ {
		result[i-s] = cloneBytes(l.items[i])
	}
	return result
}

// Insert inserts value before or after the pivot element.
// Returns the new length, or -1 if pivot not found, 0 if list is empty.
func (l *List) Insert(before bool, pivot, value []byte) int {
	if len(l.items) == 0 {
		return 0
	}

	for i, item := range l.items {
		if bytesEqual(item, pivot) {
			pos := i
			if !before {
				pos = i + 1
			}
			// Insert at position
			l.items = append(l.items, nil)
			copy(l.items[pos+1:], l.items[pos:])
			l.items[pos] = cloneBytes(value)
			return len(l.items)
		}
	}
	return -1
}

// Rem removes count occurrences of elements equal to value.
//   - count > 0: Remove first count occurrences (head to tail)
//   - count < 0: Remove last |count| occurrences (tail to head)
//   - count == 0: Remove all occurrences
//
// Returns the number of removed elements.
func (l *List) Rem(count int, value []byte) int {
	if len(l.items) == 0 {
		return 0
	}

	removed := 0
	absCount := count
	if absCount < 0 {
		absCount = -absCount
	}

	if count >= 0 {
		// Remove from head to tail
		newItems := make([][]byte, 0, len(l.items))
		for _, item := range l.items {
			if bytesEqual(item, value) && (count == 0 || removed < absCount) {
				removed++
				continue
			}
			newItems = append(newItems, item)
		}
		l.items = newItems
	} else {
		// Remove from tail to head
		newItems := make([][]byte, 0, len(l.items))
		for i := len(l.items) - 1; i >= 0; i-- {
			if bytesEqual(l.items[i], value) && removed < absCount {
				removed++
				continue
			}
			newItems = append([][]byte{l.items[i]}, newItems...)
		}
		l.items = newItems
	}
	return removed
}

// Trim trims the list to only contain elements between start and stop (inclusive).
func (l *List) Trim(start, stop int) {
	length := len(l.items)
	if length == 0 {
		return
	}

	s := l.resolveIndex(start)
	e := l.resolveIndex(stop)

	if s < 0 {
		s = 0
	}
	if e >= length {
		e = length - 1
	}
	if s > e || s >= length {
		l.items = l.items[:0]
		return
	}

	l.items = l.items[s : e+1]
}

// resolveIndex converts a possibly-negative index to a non-negative one.
func (l *List) resolveIndex(index int) int {
	if index < 0 {
		return len(l.items) + index
	}
	return index
}

// Helper: deep clone bytes
func cloneBytes(b []byte) []byte {
	if b == nil {
		return nil
	}
	c := make([]byte, len(b))
	copy(c, b)
	return c
}

// Helper: compare byte slices
func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
