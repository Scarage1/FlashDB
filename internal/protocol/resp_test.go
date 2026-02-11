package protocol

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestReader_SimpleString(t *testing.T) {
	input := "+OK\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeSimpleString), val.Type)
	assert.Equal(t, "OK", val.Str)
}

func TestReader_Error(t *testing.T) {
	input := "-ERR unknown command\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeError), val.Type)
	assert.Equal(t, "ERR unknown command", val.Str)
}

func TestReader_Integer(t *testing.T) {
	input := ":1000\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeInteger), val.Type)
	assert.Equal(t, int64(1000), val.Num)
}

func TestReader_NegativeInteger(t *testing.T) {
	input := ":-100\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeInteger), val.Type)
	assert.Equal(t, int64(-100), val.Num)
}

func TestReader_BulkString(t *testing.T) {
	input := "$5\r\nhello\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeBulkString), val.Type)
	assert.Equal(t, "hello", val.Str)
	assert.False(t, val.Null)
}

func TestReader_NullBulkString(t *testing.T) {
	input := "$-1\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeBulkString), val.Type)
	assert.True(t, val.Null)
}

func TestReader_EmptyBulkString(t *testing.T) {
	input := "$0\r\n\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeBulkString), val.Type)
	assert.Equal(t, "", val.Str)
	assert.False(t, val.Null)
}

func TestReader_BulkStringTooLarge(t *testing.T) {
	input := "$536870913\r\n"
	r := NewReader(bytes.NewBufferString(input))

	_, err := r.ReadValue()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidProtocol)
}

func TestReader_Array(t *testing.T) {
	input := "*2\r\n$3\r\nGET\r\n$3\r\nkey\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeArray), val.Type)
	require.Len(t, val.Array, 2)
	assert.Equal(t, "GET", val.Array[0].Str)
	assert.Equal(t, "key", val.Array[1].Str)
}

func TestReader_NullArray(t *testing.T) {
	input := "*-1\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeArray), val.Type)
	assert.True(t, val.Null)
}

func TestReader_EmptyArray(t *testing.T) {
	input := "*0\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeArray), val.Type)
	assert.Empty(t, val.Array)
	assert.False(t, val.Null)
}

func TestReader_ArrayTooLarge(t *testing.T) {
	input := "*1000001\r\n"
	r := NewReader(bytes.NewBufferString(input))

	_, err := r.ReadValue()
	require.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidProtocol)
}

func TestReader_NestedArray(t *testing.T) {
	input := "*2\r\n*2\r\n$1\r\na\r\n$1\r\nb\r\n*2\r\n$1\r\nc\r\n$1\r\nd\r\n"
	r := NewReader(bytes.NewBufferString(input))

	val, err := r.ReadValue()
	require.NoError(t, err)
	assert.Equal(t, byte(TypeArray), val.Type)
	require.Len(t, val.Array, 2)
	require.Len(t, val.Array[0].Array, 2)
	require.Len(t, val.Array[1].Array, 2)
}

func TestWriter_SimpleString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteSimpleString("OK")
	require.NoError(t, err)
	assert.Equal(t, "+OK\r\n", buf.String())
}

func TestWriter_Error(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteError("unknown command")
	require.NoError(t, err)
	assert.Equal(t, "-ERR unknown command\r\n", buf.String())
}

func TestWriter_Integer(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteInteger(1000)
	require.NoError(t, err)
	assert.Equal(t, ":1000\r\n", buf.String())
}

func TestWriter_BulkString(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteBulkString([]byte("hello"))
	require.NoError(t, err)
	assert.Equal(t, "$5\r\nhello\r\n", buf.String())
}

func TestWriter_Null(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteNull()
	require.NoError(t, err)
	assert.Equal(t, "$-1\r\n", buf.String())
}

func TestWriter_Array(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteArray([][]byte{[]byte("hello"), []byte("world")})
	require.NoError(t, err)
	assert.Equal(t, "*2\r\n$5\r\nhello\r\n$5\r\nworld\r\n", buf.String())
}

func TestWriter_EmptyArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteArray([][]byte{})
	require.NoError(t, err)
	assert.Equal(t, "*0\r\n", buf.String())
}

func TestWriter_StringArray(t *testing.T) {
	var buf bytes.Buffer
	w := NewWriter(&buf)

	err := w.WriteStringArray([]string{"key1", "key2"})
	require.NoError(t, err)
	assert.Equal(t, "*2\r\n$4\r\nkey1\r\n$4\r\nkey2\r\n", buf.String())
}
