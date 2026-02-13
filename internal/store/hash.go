// Package store - Hash data type implementation for FlashDB
//
// A Hash is a map of field→value pairs stored under a single key.
// Equivalent to Redis Hashes. All operations are O(1) for single-field
// operations and O(N) for bulk operations where N is the number of fields.
package store

// Hash represents a Redis-like hash data structure.
// It stores field-value pairs under a single key.
// The Hash itself is NOT thread-safe; concurrency is managed by the Store.
type Hash struct {
	fields map[string][]byte
}

// NewHash creates a new empty Hash.
func NewHash() *Hash {
	return &Hash{
		fields: make(map[string][]byte),
	}
}

// Set sets field to value. Returns true if the field is new (didn't exist before).
func (h *Hash) Set(field string, value []byte) bool {
	_, existed := h.fields[field]
	h.fields[field] = append([]byte(nil), value...)
	return !existed
}

// SetNX sets field to value only if field does not exist.
// Returns true if the field was set, false if it already existed.
func (h *Hash) SetNX(field string, value []byte) bool {
	if _, exists := h.fields[field]; exists {
		return false
	}
	h.fields[field] = append([]byte(nil), value...)
	return true
}

// Get returns the value of a field.
func (h *Hash) Get(field string) ([]byte, bool) {
	val, exists := h.fields[field]
	if !exists {
		return nil, false
	}
	result := make([]byte, len(val))
	copy(result, val)
	return result, true
}

// Del removes one or more fields. Returns the number of fields removed.
func (h *Hash) Del(fields ...string) int {
	removed := 0
	for _, f := range fields {
		if _, exists := h.fields[f]; exists {
			delete(h.fields, f)
			removed++
		}
	}
	return removed
}

// Exists returns whether a field exists in the hash.
func (h *Hash) Exists(field string) bool {
	_, exists := h.fields[field]
	return exists
}

// Len returns the number of fields in the hash.
func (h *Hash) Len() int {
	return len(h.fields)
}

// GetAll returns all field-value pairs in the hash.
// Fields and values alternate: [field1, val1, field2, val2, ...]
func (h *Hash) GetAll() []HashFieldValue {
	result := make([]HashFieldValue, 0, len(h.fields))
	for field, value := range h.fields {
		val := make([]byte, len(value))
		copy(val, value)
		result = append(result, HashFieldValue{Field: field, Value: val})
	}
	return result
}

// Keys returns all field names in the hash.
func (h *Hash) Keys() []string {
	keys := make([]string, 0, len(h.fields))
	for field := range h.fields {
		keys = append(keys, field)
	}
	return keys
}

// Vals returns all values in the hash.
func (h *Hash) Vals() [][]byte {
	vals := make([][]byte, 0, len(h.fields))
	for _, value := range h.fields {
		val := make([]byte, len(value))
		copy(val, value)
		vals = append(vals, val)
	}
	return vals
}

// IncrBy increments the integer value of a field by delta.
// If the field doesn't exist, it's initialized to 0 before incrementing.
// Returns the new value and an error if the value is not an integer.
func (h *Hash) IncrBy(field string, delta int64) (int64, error) {
	var current int64
	if val, exists := h.fields[field]; exists {
		parsed, err := parseInt64(val)
		if err != nil {
			return 0, err
		}
		current = parsed
	}

	newVal := current + delta
	h.fields[field] = formatInt64(newVal)
	return newVal, nil
}

// IncrByFloat increments the float value of a field by delta.
// If the field doesn't exist, it's initialized to 0 before incrementing.
// Returns the new value and an error if the value is not a float.
func (h *Hash) IncrByFloat(field string, delta float64) (float64, error) {
	var current float64
	if val, exists := h.fields[field]; exists {
		parsed, err := parseFloat64(val)
		if err != nil {
			return 0, err
		}
		current = parsed
	}

	newVal := current + delta
	h.fields[field] = formatFloat64(newVal)
	return newVal, nil
}

// HashFieldValue represents a field-value pair in a hash.
type HashFieldValue struct {
	Field string
	Value []byte
}
