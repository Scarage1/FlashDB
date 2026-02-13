// Package wal provides a Write-Ahead Log implementation for durability.
// Records are encoded in little-endian format with CRC32 checksums.
package wal

import (
	"encoding/binary"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"os"
	"path/filepath"
	"sync"
)

// Operation types for WAL records
const (
	OpSet        byte = 0x01
	OpDelete     byte = 0x02
	OpSetWithTTL byte = 0x03
	OpExpire     byte = 0x04
	OpPersist    byte = 0x05

	// Sorted set operations
	OpZAdd             byte = 0x10
	OpZRem             byte = 0x11
	OpZIncrBy          byte = 0x12
	OpZRemRangeByRank  byte = 0x13
	OpZRemRangeByScore byte = 0x14

	// Hash operations
	OpHSet byte = 0x20
	OpHDel byte = 0x21

	// List operations
	OpLPush byte = 0x30
	OpRPush byte = 0x31
	OpLPop  byte = 0x32
	OpRPop  byte = 0x33
	OpLSet  byte = 0x34
	OpLTrim byte = 0x35

	// Set operations
	OpSAdd byte = 0x40
	OpSRem byte = 0x41
	OpSPop byte = 0x42

	// Time-series operations
	OpTSAdd byte = 0x50
	OpTSDel byte = 0x51
)

// Header size: CRC32 (4) + Type (1) + KeyLen (4) + ValueLen (4) + TTL (8) = 21 bytes
const headerSize = 21

var (
	// ErrCorruptedRecord indicates a CRC32 mismatch in a WAL record
	ErrCorruptedRecord = errors.New("wal: corrupted record (CRC32 mismatch)")
	// ErrInvalidOperation indicates an unknown operation type
	ErrInvalidOperation = errors.New("wal: invalid operation type")
)

// Record represents a WAL record
type Record struct {
	Type     byte
	Key      []byte
	Value    []byte
	ExpireAt int64 // Unix timestamp in milliseconds, 0 means no expiration
}

// bufPool pools byte slices used for record encoding to reduce allocations
// during high-throughput batch writes.
var bufPool = sync.Pool{
	New: func() interface{} {
		b := make([]byte, 0, 4096)
		return &b
	},
}

// WAL represents a Write-Ahead Log
type WAL struct {
	mu       sync.Mutex
	file     *os.File
	filePath string
}

// Open opens or creates a WAL file at the specified path.
// If the directory doesn't exist, it will be created.
func Open(path string) (*WAL, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("wal: failed to create directory: %w", err)
	}

	// Open with RDWR for recovery, we'll seek to end after recovery
	file, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0644)
	if err != nil {
		return nil, fmt.Errorf("wal: failed to open file: %w", err)
	}

	return &WAL{
		file:     file,
		filePath: path,
	}, nil
}

// Append writes a record to the WAL.
// The record is synced to disk before returning.
func (w *WAL) Append(rec Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to end before writing
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("wal: failed to seek to end: %w", err)
	}

	data := encodeRecord(rec)
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("wal: failed to write record: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: failed to sync: %w", err)
	}

	return nil
}

// AppendBatch writes multiple records to the WAL atomically.
// All records are written into a pooled buffer and flushed + synced once,
// which is more efficient and ensures atomicity for multi-key operations.
func (w *WAL) AppendBatch(records []Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("wal: failed to seek to end: %w", err)
	}

	// Accumulate into a pooled buffer so we issue a single write syscall.
	bp := bufPool.Get().(*[]byte)
	buf := (*bp)[:0]
	for _, rec := range records {
		buf = appendEncodedRecord(buf, rec)
	}
	_, err := w.file.Write(buf)
	*bp = buf
	bufPool.Put(bp)
	if err != nil {
		return fmt.Errorf("wal: failed to write records: %w", err)
	}

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: failed to sync: %w", err)
	}

	return nil
}

// AppendNoSync writes a record to the WAL without calling fsync.
// The caller must call Sync() explicitly when the batch is complete.
// This is useful in pipeline mode where many commands arrive in quick
// succession and a single fsync at the end is sufficient.
func (w *WAL) AppendNoSync(rec Record) error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return fmt.Errorf("wal: failed to seek to end: %w", err)
	}

	data := encodeRecord(rec)
	if _, err := w.file.Write(data); err != nil {
		return fmt.Errorf("wal: failed to write record: %w", err)
	}
	return nil
}

// Sync flushes the WAL file to durable storage.
func (w *WAL) Sync() error {
	w.mu.Lock()
	defer w.mu.Unlock()
	return w.file.Sync()
}

// ReadAll reads all valid records from the WAL.
// Returns records up to the first corrupted or partial record.
// The WAL file is truncated to remove any partial records.
func (w *WAL) ReadAll() ([]Record, error) {
	w.mu.Lock()
	defer w.mu.Unlock()

	// Seek to beginning
	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return nil, fmt.Errorf("wal: failed to seek: %w", err)
	}

	var records []Record
	var validOffset int64 = 0

	for {
		rec, bytesRead, err := readRecord(w.file)
		if err != nil {
			if err == io.EOF {
				break
			}
			// Partial or corrupted record - stop reading
			break
		}
		records = append(records, rec)
		validOffset += int64(bytesRead)
	}

	// Truncate to last valid record
	if err := w.file.Truncate(validOffset); err != nil {
		return nil, fmt.Errorf("wal: failed to truncate: %w", err)
	}

	// Seek to end for appending
	if _, err := w.file.Seek(0, io.SeekEnd); err != nil {
		return nil, fmt.Errorf("wal: failed to seek to end: %w", err)
	}

	return records, nil
}

// Close closes the WAL file.
func (w *WAL) Close() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Sync(); err != nil {
		return fmt.Errorf("wal: failed to sync on close: %w", err)
	}
	return w.file.Close()
}

// Clear truncates the WAL file, removing all records.
func (w *WAL) Clear() error {
	w.mu.Lock()
	defer w.mu.Unlock()

	if err := w.file.Truncate(0); err != nil {
		return fmt.Errorf("wal: failed to truncate: %w", err)
	}

	if _, err := w.file.Seek(0, io.SeekStart); err != nil {
		return fmt.Errorf("wal: failed to seek: %w", err)
	}

	return w.file.Sync()
}

// appendEncodedRecord appends the encoded form of rec to dst (growing the
// slice as needed) and returns the extended slice. This lets AppendBatch
// accumulate many records into a single pooled buffer.
func appendEncodedRecord(dst []byte, rec Record) []byte {
	keyLen := len(rec.Key)
	valueLen := len(rec.Value)
	totalLen := headerSize + keyLen + valueLen

	// Grow dst to fit the new record.
	off := len(dst)
	if cap(dst)-off < totalLen {
		grown := make([]byte, off, off+totalLen+512) // extra headroom
		copy(grown, dst)
		dst = grown
	}
	dst = dst[:off+totalLen]
	data := dst[off:]

	data[4] = rec.Type
	binary.LittleEndian.PutUint32(data[5:9], uint32(keyLen))
	binary.LittleEndian.PutUint32(data[9:13], uint32(valueLen))
	binary.LittleEndian.PutUint64(data[13:21], uint64(rec.ExpireAt))
	copy(data[21:21+keyLen], rec.Key)
	copy(data[21+keyLen:], rec.Value)

	checksum := crc32.ChecksumIEEE(data[4:])
	binary.LittleEndian.PutUint32(data[0:4], checksum)

	return dst
}

// encodeRecord encodes a record into bytes with CRC32 checksum.
// Format: CRC32 (4) + Type (1) + KeyLen (4) + ValueLen (4) + ExpireAt (8) + Key + Value
func encodeRecord(rec Record) []byte {
	keyLen := len(rec.Key)
	valueLen := len(rec.Value)
	totalLen := headerSize + keyLen + valueLen

	data := make([]byte, totalLen)

	// Skip CRC32 for now, write other fields first
	data[4] = rec.Type
	binary.LittleEndian.PutUint32(data[5:9], uint32(keyLen))
	binary.LittleEndian.PutUint32(data[9:13], uint32(valueLen))
	binary.LittleEndian.PutUint64(data[13:21], uint64(rec.ExpireAt))
	copy(data[21:21+keyLen], rec.Key)
	copy(data[21+keyLen:], rec.Value)

	// Calculate CRC32 over everything except the CRC32 field itself
	checksum := crc32.ChecksumIEEE(data[4:])
	binary.LittleEndian.PutUint32(data[0:4], checksum)

	return data
}

// readRecord reads a single record from the reader.
// Returns the record, number of bytes read, and any error.
func readRecord(r io.Reader) (Record, int, error) {
	header := make([]byte, headerSize)
	n, err := io.ReadFull(r, header)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return Record{}, n, io.EOF
		}
		return Record{}, n, err
	}

	storedCRC := binary.LittleEndian.Uint32(header[0:4])
	recType := header[4]
	keyLen := binary.LittleEndian.Uint32(header[5:9])
	valueLen := binary.LittleEndian.Uint32(header[9:13])
	expireAt := int64(binary.LittleEndian.Uint64(header[13:21]))

	// Sanity check on lengths to prevent OOM
	if keyLen > 1<<20 || valueLen > 1<<30 {
		return Record{}, n, ErrCorruptedRecord
	}

	data := make([]byte, keyLen+valueLen)
	dataRead, err := io.ReadFull(r, data)
	if err != nil {
		if err == io.EOF || err == io.ErrUnexpectedEOF {
			return Record{}, n + dataRead, io.EOF
		}
		return Record{}, n + dataRead, err
	}

	// Verify CRC32
	crcData := make([]byte, 1+4+4+8+len(data))
	crcData[0] = recType
	binary.LittleEndian.PutUint32(crcData[1:5], keyLen)
	binary.LittleEndian.PutUint32(crcData[5:9], valueLen)
	binary.LittleEndian.PutUint64(crcData[9:17], uint64(expireAt))
	copy(crcData[17:], data)

	calculatedCRC := crc32.ChecksumIEEE(crcData)
	if calculatedCRC != storedCRC {
		return Record{}, n + dataRead, ErrCorruptedRecord
	}

	return Record{
		Type:     recType,
		Key:      data[:keyLen],
		Value:    data[keyLen:],
		ExpireAt: expireAt,
	}, headerSize + int(keyLen+valueLen), nil
}
