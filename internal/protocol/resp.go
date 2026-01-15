// Package protocol implements the RESP (Redis Serialization Protocol) parser and encoder.
package protocol

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
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

// Reader wraps a bufio.Reader for RESP parsing
type Reader struct {
	rd *bufio.Reader
}

// NewReader creates a new RESP Reader
func NewReader(r io.Reader) *Reader {
	return &Reader{rd: bufio.NewReader(r)}
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

// Writer wraps a bufio.Writer for RESP encoding
type Writer struct {
	wr *bufio.Writer
}

// NewWriter creates a new RESP Writer
func NewWriter(w io.Writer) *Writer {
	return &Writer{wr: bufio.NewWriter(w)}
}

// WriteSimpleString writes a simple string response
func (w *Writer) WriteSimpleString(s string) error {
	_, err := fmt.Fprintf(w.wr, "+%s\r\n", s)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteError writes an error response
func (w *Writer) WriteError(msg string) error {
	_, err := fmt.Fprintf(w.wr, "-ERR %s\r\n", msg)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteInteger writes an integer response
func (w *Writer) WriteInteger(n int64) error {
	_, err := fmt.Fprintf(w.wr, ":%d\r\n", n)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteBulkString writes a bulk string response
func (w *Writer) WriteBulkString(s []byte) error {
	_, err := fmt.Fprintf(w.wr, "$%d\r\n", len(s))
	if err != nil {
		return err
	}
	_, err = w.wr.Write(s)
	if err != nil {
		return err
	}
	_, err = w.wr.WriteString("\r\n")
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteNull writes a null bulk string response
func (w *Writer) WriteNull() error {
	_, err := w.wr.WriteString("$-1\r\n")
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteArray writes an array of bulk strings
func (w *Writer) WriteArray(items [][]byte) error {
	_, err := fmt.Fprintf(w.wr, "*%d\r\n", len(items))
	if err != nil {
		return err
	}
	for _, item := range items {
		_, err = fmt.Fprintf(w.wr, "$%d\r\n", len(item))
		if err != nil {
			return err
		}
		_, err = w.wr.Write(item)
		if err != nil {
			return err
		}
		_, err = w.wr.WriteString("\r\n")
		if err != nil {
			return err
		}
	}
	return w.wr.Flush()
}

// WriteStringArray writes an array of strings
func (w *Writer) WriteStringArray(items []string) error {
	byteItems := make([][]byte, len(items))
	for i, item := range items {
		byteItems[i] = []byte(item)
	}
	return w.WriteArray(byteItems)
}

// WriteArrayWithNulls writes an array where some elements may be null
func (w *Writer) WriteArrayWithNulls(items [][]byte, nulls []bool) error {
	_, err := fmt.Fprintf(w.wr, "*%d\r\n", len(items))
	if err != nil {
		return err
	}
	for i, item := range items {
		if nulls[i] {
			_, err = w.wr.WriteString("$-1\r\n")
		} else {
			_, err = fmt.Fprintf(w.wr, "$%d\r\n", len(item))
			if err != nil {
				return err
			}
			_, err = w.wr.Write(item)
			if err != nil {
				return err
			}
			_, err = w.wr.WriteString("\r\n")
		}
		if err != nil {
			return err
		}
	}
	return w.wr.Flush()
}

// WriteRaw writes raw bytes directly to the connection
func (w *Writer) WriteRaw(data []byte) error {
	_, err := w.wr.Write(data)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}

// WriteArrayHeader writes only the array header (for incremental array building)
func (w *Writer) WriteArrayHeader(count int) error {
	_, err := fmt.Fprintf(w.wr, "*%d\r\n", count)
	if err != nil {
		return err
	}
	return w.wr.Flush()
}
