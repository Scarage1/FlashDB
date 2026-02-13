# FlashDB Architecture Document

> **Internal Engineering Reference**  
> Version 2.0 | February 2026

---

## 1. System Architecture

### 1.1 Current Architecture
```
                    ┌──────────────────────────────┐
                    │         Clients               │
                    │  redis-cli / app / web UI     │
                    └──────────┬───────────────────┘
                               │
              ┌────────────────┼────────────────┐
              │                │                │
        ┌─────┴─────┐  ┌──────┴──────┐  ┌──────┴──────┐
        │ RESP TCP   │  │ HTTP REST   │  │ WebSocket   │
        │ :6379      │  │ :8080       │  │ (planned)   │
        └─────┬─────┘  └──────┬──────┘  └──────┬──────┘
              │                │                │
              └────────────────┼────────────────┘
                               │
                    ┌──────────┴──────────┐
                    │   Command Router     │
                    │   (server.go)        │
                    └──────────┬──────────┘
                               │
                    ┌──────────┴──────────┐
                    │      Engine          │
                    │   (engine.go)        │
                    │  Coordinates WAL     │
                    │  + Store access      │
                    └────┬──────────┬─────┘
                         │          │
              ┌──────────┴──┐  ┌───┴──────────┐
              │ In-Memory   │  │    WAL        │
              │ Store       │  │  (wal.go)     │
              │ (store.go)  │  │  Disk-backed  │
              │             │  │  append-only   │
              │ ┌─────────┐ │  └───────────────┘
              │ │ KV Data  │ │
              │ ├─────────┤ │
              │ │ Sorted   │ │
              │ │ Sets     │ │
              │ ├─────────┤ │
              │ │ TTL GC   │ │
              │ └─────────┘ │
              └─────────────┘
```

### 1.2 Target Architecture (Post-Roadmap)
```
                    ┌──────────────────────────────────────┐
                    │              Clients                   │
                    └──────────────┬───────────────────────┘
                                   │
              ┌────────────────────┼──────────────────────┐
              │                    │                       │
        ┌─────┴─────┐  ┌──────────┴────────┐  ┌──────────┴────┐
        │ RESP TCP   │  │  HTTP/REST API    │  │  WebSocket     │
        │ + TLS      │  │  + Auth + CORS    │  │  Real-time     │
        │ :6379      │  │  :8080            │  │  :8080/ws      │
        └─────┬─────┘  └──────────┬────────┘  └──────────┬────┘
              │                    │                       │
              └────────────────────┼───────────────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │     Authentication Layer     │
                    │   ACL / Token / Password     │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │      Command Router          │
                    │  + Pipeline Support           │
                    │  + Rate Limiting              │
                    │  + Slow Query Log             │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │         Engine               │
                    │   Sharded Lock Manager       │
                    └──┬────┬────┬────┬────┬──────┘
                       │    │    │    │    │
                    ┌──┴┐┌──┴┐┌──┴┐┌──┴┐┌──┴──┐
                    │KV ││Hash││List││Set││ZSet │
                    │   ││    ││    ││   ││     │
                    └───┘└────┘└────┘└───┘└─────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │     WAL Manager              │
                    │  Segmented + Compaction       │
                    │  + Snapshot support           │
                    └──────────────┬──────────────┘
                                   │
                    ┌──────────────┴──────────────┐
                    │        Disk Storage          │
                    │  data/                        │
                    │  ├── flashdb-00000001.wal     │
                    │  ├── flashdb-00000002.wal     │
                    │  └── snapshots/               │
                    └─────────────────────────────┘
```

---

## 2. Package Structure

### Current
```
flashdb/
├── cmd/
│   ├── flashdb/          # Main server binary
│   ├── flashdb-benchmark/ # Benchmark tool
│   └── test-client/       # Test client
├── internal/
│   ├── config/    # Configuration (currently unused)
│   ├── engine/    # Storage engine coordinator
│   ├── protocol/  # RESP protocol parser/encoder
│   ├── server/    # TCP server + command handlers
│   ├── store/     # In-memory data store
│   ├── wal/       # Write-ahead log
│   └── web/       # HTTP API server
├── frontend/      # Next.js dashboard
└── docs/          # Documentation
```

### Target
```
flashdb/
├── cmd/
│   ├── flashdb/           # Main server binary
│   ├── flashdb-cli/       # Official CLI tool
│   └── flashdb-benchmark/ # Benchmark tool
├── internal/
│   ├── auth/       # ACL, authentication, TLS
│   ├── cluster/    # Clustering, replication
│   ├── config/     # Unified configuration
│   ├── engine/     # Storage engine coordinator
│   ├── metrics/    # Prometheus metrics, slow log
│   ├── protocol/   # RESP protocol parser/encoder
│   ├── server/     # TCP server + command router
│   ├── store/      # In-memory data structures
│   │   ├── kv.go        # String key-value
│   │   ├── hash.go      # Hash maps
│   │   ├── list.go      # Linked lists
│   │   ├── set.go       # Sets
│   │   ├── sortedset.go # Sorted sets
│   │   └── timeseries.go # Time-series (advanced)
│   ├── wal/        # Segmented WAL + compaction
│   └── web/        # HTTP API + WebSocket server
├── frontend/       # Next.js professional dashboard
│   ├── src/
│   │   ├── app/           # Next.js app router pages
│   │   │   ├── (dashboard)/
│   │   │   │   ├── page.tsx        # Dashboard
│   │   │   │   ├── console/        # Terminal
│   │   │   │   ├── explorer/       # Key browser
│   │   │   │   ├── monitoring/     # Metrics
│   │   │   │   └── settings/       # Config
│   │   │   └── layout.tsx
│   │   ├── components/
│   │   │   ├── ui/        # Primitives (Button, Input, etc.)
│   │   │   ├── charts/    # Chart components
│   │   │   ├── console/   # Terminal components
│   │   │   ├── explorer/  # Key browser components
│   │   │   └── layout/    # Shell, Sidebar, TopBar
│   │   ├── hooks/         # Custom React hooks
│   │   ├── lib/           # API client, utilities
│   │   └── stores/        # Zustand state stores
│   └── public/
└── docs/
    ├── ARCHITECTURE.md     # This file
    ├── COMMANDS.md         # Command reference
    ├── PROTOCOL.md         # Wire protocol
    └── API.md              # HTTP API reference
```

---

## 3. Data Flow

### Write Path (SET key value)
```
Client → RESP Parser → Command Router → Auth Check
  → Engine.Set()
    → [Lock shard for key]
    → WAL.Append(SetRecord)  ← disk sync
    → Store.Set(key, value)  ← in-memory
    → [Unlock shard]
  → RESP Writer → "+OK\r\n" → Client
```

### Read Path (GET key)
```
Client → RESP Parser → Command Router → Auth Check
  → Engine.Get()
    → [RLock shard for key]
    → Store.Get(key)
      → Check TTL (lazy expiration)
      → Return value copy
    → [RUnlock shard]
  → RESP Writer → "$5\r\nvalue\r\n" → Client
```

### Transaction Path (MULTI → SET → GET → EXEC)
```
MULTI  → Set client.inMulti = true
SET    → Queue command (don't execute)
GET    → Queue command (don't execute)
EXEC   → [Lock ALL relevant shards]
       → Execute SET (WAL + memory)
       → Execute GET (memory read)
       → [Unlock ALL shards]
       → Return array of results
```

---

## 4. WAL Design (Target)

### Segment-Based WAL
```
data/
├── wal/
│   ├── 00000001.wal    (64 MiB max)
│   ├── 00000002.wal    (64 MiB max)
│   └── 00000003.wal    (active, being written to)
└── snapshots/
    └── snapshot-1708000000.rdb  (periodic full dump)
```

### Compaction Strategy
1. **Periodic snapshots**: Every N minutes, dump full state to disk
2. **WAL truncation**: After snapshot, delete WAL segments older than snapshot
3. **Recovery**: Load snapshot → replay WAL segments after snapshot timestamp

### Record Format (Extended)
```
┌─────────┬──────┬────────┬──────────┬──────────┬─────┬───────┐
│ CRC32   │ Type │ KeyLen │ ValueLen │ ExpireAt │ Key │ Value │
│ 4 bytes │ 1 B  │ 4 B   │ 4 B     │ 8 B     │ var │ var   │
└─────────┴──────┴────────┴──────────┴──────────┴─────┴───────┘

New Types:
  0x01 = OpSet
  0x02 = OpDelete
  0x03 = OpSetWithTTL
  0x04 = OpExpire
  0x05 = OpPersist
  0x10 = OpZAdd        (NEW)
  0x11 = OpZRem        (NEW)
  0x12 = OpZIncrBy     (NEW)
  0x20 = OpHSet        (NEW)
  0x21 = OpHDel        (NEW)
  0x30 = OpLPush       (NEW)
  0x31 = OpRPush       (NEW)
  0x32 = OpLPop        (NEW)
  0x33 = OpRPop        (NEW)
  0x40 = OpSAdd        (NEW)
  0x41 = OpSRem        (NEW)
```

---

## 5. Concurrency Model

### Current: Single Global Mutex
```
Engine.mu (RWMutex)
  └── ALL operations go through this
      └── Store.mu (RWMutex)  ← redundant double lock
          └── SortedSet.mu (RWMutex)  ← triple lock!
```

### Target: Sharded Locking
```
Engine (no global lock)
  └── ShardedStore
      ├── Shard[0].mu → keys hashing to shard 0
      ├── Shard[1].mu → keys hashing to shard 1
      ├── ...
      └── Shard[255].mu → keys hashing to shard 255

Multi-key operations:
  → Sort shard IDs to prevent deadlock
  → Lock shards in order
  → Execute
  → Unlock in reverse order
```

---

## 6. Frontend Architecture

### State Management
```
Zustand Stores
├── useServerStore     # Connection status, server info
├── useConsoleStore    # Command history, output buffer
├── useExplorerStore   # Keys list, selected key, filters
├── useMonitorStore    # Metrics history, alerts
└── useSettingsStore   # Theme, preferences, config
```

### WebSocket Integration
```typescript
// Real-time event stream from server
const ws = new WebSocket('ws://localhost:8080/ws');

ws.onmessage = (event) => {
  const msg = JSON.parse(event.data);
  switch (msg.type) {
    case 'stats':     useServerStore.getState().updateStats(msg.data); break;
    case 'command':   useMonitorStore.getState().addCommand(msg.data); break;
    case 'keyspace':  useExplorerStore.getState().invalidate(); break;
    case 'alert':     useMonitorStore.getState().addAlert(msg.data); break;
  }
};
```

### Component Design System
All UI primitives follow:
- **Radix UI** for accessibility (keyboard nav, ARIA, focus management)
- **Tailwind CSS** for styling (no CSS-in-JS)
- **Framer Motion** for animations
- **Consistent API**: all components accept `className`, `variant`, `size`

---

## 7. API Design

### REST API (Versioned)
```
GET    /api/v1/stats          # Server statistics
GET    /api/v1/keys           # List keys (with pagination)
GET    /api/v1/key/:key       # Get key details
PUT    /api/v1/key/:key       # Set key value
DELETE /api/v1/key/:key       # Delete key
POST   /api/v1/execute        # Execute raw command
GET    /api/v1/healthz        # Health check
GET    /api/v1/readyz         # Readiness check
WS     /api/v1/ws             # WebSocket stream

# Monitoring
GET    /api/v1/clients        # Connected clients
GET    /api/v1/slowlog        # Slow query log
GET    /api/v1/metrics        # Prometheus metrics

# Management
GET    /api/v1/config         # Server configuration
PUT    /api/v1/config         # Update configuration
POST   /api/v1/snapshot       # Create snapshot
GET    /api/v1/snapshots      # List snapshots
```

### Error Response Format
```json
{
  "success": false,
  "error": {
    "code": "KEY_NOT_FOUND",
    "message": "The key 'user:123' does not exist",
    "details": {}
  },
  "request_id": "req_abc123"
}
```

---

## 8. Testing Strategy

### Test Pyramid
```
         ┌─────┐
         │ E2E │  ← Docker-based integration
         │ 10% │     (redis-cli compatibility)
        ┌┴─────┴┐
        │ Integ │  ← TCP server tests
        │  30%  │     (server_test.go)
       ┌┴───────┴┐
       │  Unit   │  ← Package-level tests
       │  60%    │     (store, engine, wal, protocol)
       └─────────┘
```

### Benchmark Requirements
- SET: >200K ops/sec (single core)
- GET: >300K ops/sec (single core)
- Mixed: >150K ops/sec (50 clients)
- Latency P99: <1ms
- Recovery: <5s for 1M keys

---

*This architecture document should be updated as design decisions evolve.*
