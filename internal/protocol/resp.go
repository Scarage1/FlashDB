// Package protocol implements the RESP (Redis Serialization Protocol) parser and encoder.
package protocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"sync"
)

var (
	// ErrInvalidProtocol indicates malformed RESP data
	ErrInvalidProtocol = errors.New("protocol: invalid RESP format")
	// ErrUnexpectedType indicates an unexpected RESP type
	ErrUnexpectedType = errors.New("protocol: unexpected type")
)

// Value represents a RESP value
type Value struct {
	Type  byte
	Str   string
	Num   int64
	Array []Value
	Null  bool
}

// RESP type constants
const (
	TypeSimpleString = '+'
	TypeError        = '-'
	TypeInteger      = ':'
	TypeBulkString   = '$'
	TypeArray        = '*'
)

const (
	maxBulkStringLength = 512 * 1024 * 1024 // 512 MiB
	maxArrayLength      = 1_000_000
	defaultBufSize      = 64 * 1024 // 64 KiB read/write buffers
)

// Shared byte slices to avoid allocations on every write.
var (
	crlfBytes = []byte("\r\n")
	nullBytes = []byte("$-1\r\n")
	errPrefix = []byte("-ERR ")
	okBytes   = []byte("+OK\r\n")
)

// intBufPool provides scratch buffers for integer formatting.
var intBufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 20) // max int64 is 19 digits + sign
		return &b
	},
}

// Reader wraps a bufio.Reader for RESP parsing
type Reader struct {
	rd *bufio.Reader
}

// NewReader creates a new RESP Reader with an optimised buffer.
func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReaderSize(r, defaultBufSize)}
}

// Buffered returns the number of bytes that can be read from the
// underlying buffer without issuing a syscall. This is used by the
// server to detect pipelined commands.
func (r *Reader) Buffered() int {
	return r.rd.Buffered()
}

// ReadValue reads a single RESP value from the reader
func (r *Reader) ReadValue() (Value, error) {
	typeByte, err := r.rd.ReadByte()
	if err != nil {
		return Value{}, err
	}

	switch typeByte {
	case TypeSimpleString:
		return r.readSimpleString()
	case TypeError:
		return r.readError()
	case TypeInteger:
		return r.readInteger()
	case TypeBulkString:
		return r.readBulkString()
	case TypeArray:
		return r.readArray()
	default:
		return Value{}, fmt.Errorf("%w: unknown type %c", ErrInvalidProtocol, typeByte)
	}
}

// readLine reads a line until \r\n
func (r *Reader) readLine() (string, error) {
	line, err := r.rd.ReadString('\n')
	if err != nil {
		return "", err
	}
	if len(line) < 2 || line[len(line)-2] != '\r' {
		return "", ErrInvalidProtocol
	}
	return line[:len(line)-2], nil
}

func (r *Reader) readSimpleString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: TypeSimpleString, Str: line}, nil
}

func (r *Reader) readError() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	return Value{Type: TypeError, Str: line}, nil
}

func (r *Reader) readInteger() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	num, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("%w: invalid integer", ErrInvalidProtocol)
	}
	return Value{Type: TypeInteger, Num: num}, nil
}

func (r *Reader) readBulkString() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	length, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("%w: invalid bulk string length", ErrInvalidProtocol)
	}

	// Null bulk string
	if length == -1 {
		return Value{Type: TypeBulkString, Null: true}, nil
	}

	if length < 0 {
		return Value{}, fmt.Errorf("%w: negative bulk string length", ErrInvalidProtocol)
	}
	if length > maxBulkStringLength {
		return Value{}, fmt.Errorf("%w: bulk string too large", ErrInvalidProtocol)
	}

	// Read the data + \r\n
	data := make([]byte, length+2)
	_, err = io.ReadFull(r.rd, data)
	if err != nil {
		return Value{}, err
	}

	if data[length] != '\r' || data[length+1] != '\n' {
		return Value{}, ErrInvalidProtocol
	}

	return Value{Type: TypeBulkString, Str: string(data[:length])}, nil
}

func (r *Reader) readArray() (Value, error) {
	line, err := r.readLine()
	if err != nil {
		return Value{}, err
	}
	count, err := strconv.ParseInt(line, 10, 64)
	if err != nil {
		return Value{}, fmt.Errorf("%w: invalid array length", ErrInvalidProtocol)
	}

	// Null array
	if count == -1 {
		return Value{Type: TypeArray, Null: true}, nil
	}

	if count < 0 {
		return Value{}, fmt.Errorf("%w: negative array length", ErrInvalidProtocol)
	}
	if count > maxArrayLength {
		return Value{}, fmt.Errorf("%w: array too large", ErrInvalidProtocol)
	}

	array := make([]Value, count)
	for i := int64(0); i < count; i++ {
		val, err := r.ReadValue()
		if err != nil {
			return Value{}, err
		}
		array[i] = val
	}

	return Value{Type: TypeArray, Array: array}, nil
}

// Writer wraps a bufio.Writer for RESP encoding.
// By default every Write* call flushes immediately (autoFlush=true).
// Call SetAutoFlush(false) before a pipeline batch, then Flush()
// once at the end, to amortise syscalls across many responses.
type Writer struct {
	wr        *bufio.Writer
	autoFlush bool
}

// NewWriter creates a new RESP Writer with an optimised buffer.
func NewWriter(w io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriterSize(w, defaultBufSize), autoFlush: true}
}

// SetAutoFlush controls whether each Write* call flushes automatically.
// Disable it for pipeline batches and call Flush() explicitly.
func (w *Writer) SetAutoFlush(on bool) { w.autoFlush = on }

// Flush writes any buffered data to the underlying io.Writer.
func (w *Writer) Flush() error { return w.wr.Flush() }

// flush conditionally flushes based on autoFlush setting.
func (w *Writer) flush() error {
	if w.autoFlush {
		return w.wr.Flush()
	}
	return nil
}

// appendInt appends the integer n (with a preceding type byte) to the
// bufio.Writer using strconv.AppendInt instead of fmt.Fprintf.
func (w *Writer) writeTypedInt(prefix byte, n int64) error {
	if err := w.wr.WriteByte(prefix); err != nil {
		return err
	}
	bp := intBufPool.Get().(*[]byte)
	b := strconv.AppendInt((*bp)[:0], n, 10)
	_, err := w.wr.Write(b)
	*bp = b
	intBufPool.Put(bp)
	if err != nil {
		return err
	}
	_, err = w.wr.Write(crlfBytes)
	return err
}

// WriteSimpleString writes a simple string response (+OK\r\n fast-path).
func (w *Writer) WriteSimpleString(s string) error {
	if s == "OK" {
		_, err := w.wr.Write(okBytes)
		if err != nil {
			return err
		}
		return w.flush()
	}
	if err := w.wr.WriteByte('+'); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(s); err != nil {
		return err
	}
	if _, err := w.wr.Write(crlfBytes); err != nil {
		return err
	}
	return w.flush()
}

// WriteError writes an error response
func (w *Writer) WriteError(msg string) error {
	if _, err := w.wr.Write(errPrefix); err != nil {
		return err
	}
	if _, err := w.wr.WriteString(msg); err != nil {
		return err
	}
	if _, err := w.wr.Write(crlfBytes); err != nil {
		return err
	}
	return w.flush()
}

// WriteInteger writes an integer response
func (w *Writer) WriteInteger(n int64) error {
	if err := w.writeTypedInt(':', n); err != nil {
		return err
	}
	return w.flush()
}

// WriteBulkString writes a bulk string response
func (w *Writer) WriteBulkString(s []byte) error {
	if err := w.writeTypedInt('$', int64(len(s))); err != nil {
		return err
	}
	if _, err := w.wr.Write(s); err != nil {
		return err
	}
	if _, err := w.wr.Write(crlfBytes); err != nil {
		return err
	}
	return w.flush()
}

// WriteNull writes a null bulk string response
func (w *Writer) WriteNull() error {
	if _, err := w.wr.Write(nullBytes); err != nil {
		return err
	}
	return w.flush()
}

// WriteArray writes an array of bulk strings
func (w *Writer) WriteArray(items [][]byte) error {
	if err := w.writeTypedInt('*', int64(len(items))); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.writeTypedInt('$', int64(len(item))); err != nil {
			return err
		}
		if _, err := w.wr.Write(item); err != nil {
			return err
		}
		if _, err := w.wr.Write(crlfBytes); err != nil {
			return err
		}
	}
	return w.flush()
}

// WriteStringArray writes an array of strings (avoids []byte conversion allocations).
func (w *Writer) WriteStringArray(items []string) error {
	if err := w.writeTypedInt('*', int64(len(items))); err != nil {
		return err
	}
	for _, item := range items {
		if err := w.writeTypedInt('$', int64(len(item))); err != nil {
			return err
		}
		if _, err := w.wr.WriteString(item); err != nil {
			return err
		}
		if _, err := w.wr.Write(crlfBytes); err != nil {
			return err
		}
	}
	return w.flush()
}

// WriteArrayWithNulls writes an array where some elements may be null
func (w *Writer) WriteArrayWithNulls(items [][]byte, nulls []bool) error {
	if err := w.writeTypedInt('*', int64(len(items))); err != nil {
		return err
	}
	for i, item := range items {
		if nulls[i] {
			if _, err := w.wr.Write(nullBytes); err != nil {
				return err
			}
		} else {
			if err := w.writeTypedInt('$', int64(len(item))); err != nil {
				return err
			}
			if _, err := w.wr.Write(item); err != nil {
				return err
			}
			if _, err := w.wr.Write(crlfBytes); err != nil {
				return err
			}
		}
	}
	return w.flush()
}

// WriteRaw writes raw bytes directly to the connection
func (w *Writer) WriteRaw(data []byte) error {
	_, err := w.wr.Write(data)
	if err != nil {
		return err
	}
	return w.flush()
}

// WriteArrayHeader writes only the array header (for incremental array building)
func (w *Writer) WriteArrayHeader(count int) error {
	if err := w.writeTypedInt('*', int64(count)); err != nil {
		return err
	}
	return w.flush()
}
