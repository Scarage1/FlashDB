# Write-Ahead Log (WAL) Specification

## Overview

The WAL provides durability by persisting all write operations before applying them to the in-memory store.

## Record Format

All integers are encoded in **little-endian** format.

```
┌──────────┬──────────┬──────────┬──────────────┬──────────┐
│  CRC32   │   Type   │  KeyLen  │   ValueLen   │   Data   │
│ 4 bytes  │  1 byte  │ 4 bytes  │   4 bytes    │ variable │
└──────────┴──────────┴──────────┴──────────────┴──────────┘
```

### Fields

| Field     | Size     | Description                              |
|-----------|----------|------------------------------------------|
| CRC32     | 4 bytes  | CRC32 checksum of Type + KeyLen + ValueLen + Data |
| Type      | 1 byte   | Operation type (1=SET, 2=DELETE)         |
| KeyLen    | 4 bytes  | Length of key in bytes                   |
| ValueLen  | 4 bytes  | Length of value in bytes (0 for DELETE)  |
| Data      | variable | Key bytes followed by Value bytes        |

### Operation Types

| Type | Value | Description          |
|------|-------|----------------------|
| SET  | 0x01  | Set key-value pair   |
| DEL  | 0x02  | Delete key           |

## CRC32 Calculation

The CRC32 is calculated over the bytes: `Type + KeyLen + ValueLen + Data`

Using IEEE polynomial (same as Go's `hash/crc32.IEEE`).

## Partial Record Handling

If a partial record is found at the end of the WAL file:
1. Log a warning
2. Truncate the file to the last valid record
3. Continue normal operation

This handles crashes during write operations.

## File Management

- WAL file: `data/flashdb.wal`
- Sync after each write for durability
- File is opened with O_APPEND | O_CREATE | O_RDWR

## Recovery Process

1. Open WAL file for reading
2. For each record:
   - Read header (CRC32 + Type + KeyLen + ValueLen)
   - If incomplete header, truncate and stop
   - Read data (key + value)
   - If incomplete data, truncate and stop
   - Verify CRC32
   - If CRC32 mismatch, truncate and stop
   - Apply operation to store
3. Reopen WAL file for appending
