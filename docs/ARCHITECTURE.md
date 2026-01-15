# FlashDB Architecture

## Overview

FlashDB is a Redis-inspired persistent distributed key-value store written in Go. It provides durability through Write-Ahead Logging (WAL) and supports concurrent client connections.

## Components

```
┌─────────────────────────────────────────────────────────────┐
│                        Client                               │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    TCP Server (RESP)                        │
│                   internal/server/                          │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Command Parser                           │
│                   internal/protocol/                        │
└─────────────────────────────────────────────────────────────┘
                              │
                              ▼
┌─────────────────────────────────────────────────────────────┐
│                    Storage Engine                           │
│                   internal/engine/                          │
└─────────────────────────────────────────────────────────────┘
                    │                   │
                    ▼                   ▼
┌──────────────────────────┐  ┌──────────────────────────────┐
│     Write-Ahead Log      │  │      In-Memory Store         │
│     internal/wal/        │  │      internal/store/         │
└──────────────────────────┘  └──────────────────────────────┘
```

## Data Flow

### Write Path
1. Client sends command via RESP protocol
2. Server parses command
3. Engine writes to WAL (append-only)
4. Engine applies to in-memory store
5. Engine responds to client

### Read Path
1. Client sends command via RESP protocol
2. Server parses command
3. Engine reads from in-memory store
4. Engine responds to client

## Thread Safety

- All engine operations are protected by a mutex
- WAL writes are serialized
- In-memory store uses sync.RWMutex for concurrent reads

## Recovery

On startup:
1. Open WAL file
2. Read and validate each record (CRC32 check)
3. Apply valid records to in-memory store
4. Truncate any partial record at end
5. Ready to serve clients
