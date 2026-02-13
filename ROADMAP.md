# FlashDB — Master Engineering Roadmap

> **Principal Engineer Assessment & Strategic Plan**  
> Document Version: 2.0 | Date: February 2026  
> Classification: Internal Engineering — Confidential

---

## Table of Contents

1. [Executive Summary](#1-executive-summary)
2. [Current State Assessment](#2-current-state-assessment)
3. [Critical Issues (P0 — Ship Blockers)](#3-critical-issues-p0)
4. [Architecture Issues (P1 — Must Fix)](#4-architecture-issues-p1)
5. [Missing Fundamentals (P2 — Expected by Users)](#5-missing-fundamentals-p2)
6. [Competitive Analysis & Differentiators](#6-competitive-analysis)
7. [Advanced Features — No Competitor Offers](#7-advanced-features)
8. [Scalability Architecture](#8-scalability-architecture)
9. [Frontend — Professional UI/UX Redesign](#9-frontend-redesign)
10. [Implementation Phases](#10-implementation-phases)
11. [Technical Specifications](#11-technical-specifications)
12. [Quality Gates](#12-quality-gates)

---

## 1. Executive Summary

FlashDB has a solid foundation: a working RESP protocol server, WAL-based persistence, sorted sets, pub/sub, and transactions. However, to become a **professional database product**, we need to address **23 backend bugs/gaps**, **8 frontend limitations**, and build **10+ industry-first features** that no competitor offers.

**The vision**: FlashDB will be the world's first **AI-native, visually intelligent key-value store** — combining Redis-level performance with a dashboard experience that rivals Vercel/Linear/Raycast in design quality.

---

## 2. Current State Assessment

### What Works Well ✅
- RESP protocol implementation (reader/writer) — solid and well-tested
- WAL with CRC32 checksums — correct binary format
- 60+ Redis commands implemented (strings, keys, sorted sets, pub/sub, transactions)
- Comprehensive test suite (engine, store, server, protocol, WAL, web)
- Clean Go project structure with proper packages
- Benchmark tooling exists
- Web API with versioned endpoints (`/api/v1/`)
- Health/readiness endpoints for container orchestration

### What's Broken or Missing ❌

| Category | Issue Count | Severity |
|----------|-----------|----------|
| Data Loss Bugs | 2 | 🔴 Critical |
| Concurrency Bugs | 3 | 🔴 Critical |
| Performance Issues | 4 | 🟠 High |
| Missing Data Types | 3 | 🟠 High |
| Security Gaps | 5 | 🟠 High |
| Scalability Blockers | 6 | 🟡 Medium |
| Frontend Limitations | 8 | 🟡 Medium |

---

## 3. Critical Issues (P0 — Ship Blockers)

### 3.1 🔴 Sorted Sets NOT Persisted to WAL
**File**: `internal/engine/engine.go` (line ~515)  
**Impact**: ALL sorted set data is LOST on server restart  
**Evidence**: Comment says `"Note: Sorted sets are currently in-memory only (not persisted to WAL)"`  
**Fix**: Add WAL operation types `OpZAdd`, `OpZRem`, `OpZIncrBy`, `OpZRemRangeByRank`, `OpZRemRangeByScore` and persist all sorted set mutations.

### 3.2 🔴 WAL Grows Unbounded
**File**: `internal/wal/wal.go`  
**Impact**: WAL file grows forever. Recovery time increases linearly. Disk will fill up.  
**Fix**: Implement WAL compaction/rotation:
- Periodic snapshot + WAL truncation
- Segment-based WAL (numbered files, rotate at 64MB)
- Background compaction thread

### 3.3 🔴 GC Loop Scans ALL Keys Every 100ms
**File**: `internal/store/store.go` (line ~53)  
**Impact**: With 1M keys, the GC loop will cause massive lock contention and CPU spikes  
**Fix**: Implement Redis-style lazy + active expiration:
- **Lazy expiration**: Check TTL on access (already partially done)
- **Active expiration**: Sample 20 random keys per cycle, delete expired ones, repeat if >25% were expired

### 3.4 🔴 MSET is Not Atomic
**File**: `internal/server/server.go`  
**Impact**: If server crashes mid-MSET, partial keys are written  
**Fix**: Batch WAL writes — write all records atomically, then apply all in-memory mutations

### 3.5 🔴 MSETNX Has Race Condition
**File**: `internal/server/server.go`  
**Impact**: Checks existence and sets via separate engine calls — another client can insert between check and set  
**Fix**: Add `engine.MSetNX()` method that holds the lock for the entire operation

### 3.6 🔴 Transaction EXEC Doesn't Provide Isolation
**File**: `internal/server/server.go` (cmdExec)  
**Impact**: Other clients can modify data between queued commands during EXEC  
**Fix**: Hold engine write lock for entire EXEC block, execute all commands atomically

---

## 4. Architecture Issues (P1 — Must Fix)

### 4.1 Single Global Mutex Bottleneck
**Problem**: Engine uses ONE `sync.RWMutex` for ALL key operations  
**Impact**: Read operations block during any write. No parallelism between unrelated keys.  
**Fix**: Implement **sharded locking** — partition keyspace into 256 shards, each with its own RWMutex. Hash key to determine shard.

### 4.2 Config System Unused
**Problem**: `internal/config/config.go` exists but `cmd/flashdb/main.go` uses flags directly  
**Fix**: Unify configuration: flags → config file → env vars → defaults (precedence order)

### 4.3 Version Hardcoded in 3 Places
**Problem**: `"1.0.0"` appears in `server.go`, `main.go`, `web.go`  
**Fix**: Single `version` package, injected via `go build -ldflags`

### 4.4 No Structured Logging
**Problem**: Uses stdlib `log` package. No levels, no JSON, no request correlation.  
**Fix**: Adopt `slog` (Go 1.21+ stdlib) with JSON handler for production

### 4.5 SortedSet Triple Locking
**Problem**: Engine mutex → Store mutex → SortedSet mutex (3 layers of locking)  
**Fix**: Remove SortedSet internal mutex (Store already serializes access). Engine should be the only lock coordinator.

### 4.6 TYPE Command Only Returns "string"
**Problem**: `cmdType()` doesn't check sorted sets  
**Fix**: Check `store.ZExists(key)` and return "zset", also return "list", "hash", "set" for future types

---

## 5. Missing Fundamentals (P2 — Expected by Users)

### 5.1 Missing Data Types
| Type | Commands Needed | Priority |
|------|----------------|----------|
| **Hash** | HSET, HGET, HMSET, HMGET, HDEL, HGETALL, HKEYS, HVALS, HLEN, HEXISTS, HINCRBY, HSCAN | 🔴 High |
| **List** | LPUSH, RPUSH, LPOP, RPOP, LRANGE, LLEN, LINDEX, LSET, LINSERT, LREM, LTRIM | 🔴 High |
| **Set** | SADD, SREM, SMEMBERS, SISMEMBER, SCARD, SINTER, SUNION, SDIFF, SRANDMEMBER, SPOP, SSCAN | 🟠 Medium |

### 5.2 Missing Key Features
- **KEYS glob matching** — Currently only supports `*`, not patterns like `user:*`
- **SCAN cursor** — Current implementation breaks on data changes between scans
- **RANDOMKEY** — Returns `keys[0]`, not random
- **Pipeline support** — Not implemented despite benchmark flag existing
- **WAIT command** — For replication acknowledgment
- **OBJECT ENCODING** — Always returns "raw", should detect int/embstr/etc.

### 5.3 Missing Server Features
- **Multiple databases** — SELECT only allows db0
- **ACL system** — Only single password, no per-user permissions
- **TLS support** — No encryption for TCP connections
- **Lua scripting** (EVAL/EVALSHA) — Critical for atomic multi-step operations
- **Slow log** — No slow query tracking
- **Keyspace notifications** — No `__keyevent@0__:expired` etc.

---

## 6. Competitive Analysis

| Feature | Redis | Dragonfly | KeyDB | Valkey | **FlashDB (Target)** |
|---------|-------|-----------|-------|--------|---------------------|
| Performance | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐ | ⭐⭐⭐⭐⭐ |
| Visual Dashboard | ❌ | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| AI Query Assistant | ❌ | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| Real-time Visualization | ❌ | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| Built-in Monitoring | ❌ (3rd party) | ❌ | ❌ | ❌ | ✅ **Native** |
| Time-Series Native | ❌ (module) | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| Schema Registry | ❌ | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| Hot Key Detection | ❌ (manual) | ❌ | ❌ | ❌ | ✅ **Automatic** |
| Natural Language Queries | ❌ | ❌ | ❌ | ❌ | ✅ **Industry-First** |
| Multi-tenant Namespaces | ❌ | ❌ | ❌ | ❌ | ✅ **Native** |

---

## 7. Advanced Features — No Competitor Offers

### 7.1 🧠 AI-Powered Query Assistant
Natural language interface: *"Show me all user sessions expiring in the next 5 minutes"* → translates to `SCAN 0 MATCH session:user:* COUNT 1000` + TTL filtering.

### 7.2 📊 Real-Time Data Flow Visualization
Live animated dashboard showing:
- Operations per second (streaming chart)
- Key access heatmap
- Memory allocation timeline
- Client connection graph

### 7.3 🔥 Automatic Hot Key Detection & Alerting
Track access frequency per key. Alert when a key exceeds threshold. Suggest caching strategies. Visual heatmap in dashboard.

### 7.4 ⏰ Native Time-Series Data Type
`TS.ADD`, `TS.GET`, `TS.RANGE`, `TS.DOWNSCALE` — built into the engine, not a module. Automatic downsampling, retention policies, aggregation functions.

### 7.5 📋 Integrated Schema Registry
Define key naming conventions and value schemas:
```
SCHEMA.SET user:{id} JSON {"name": "string", "email": "string", "age": "int"}
```
Validate on write. Schema browser in dashboard.

### 7.6 🔄 Built-in Change Data Capture (CDC)
Stream all mutations as structured events to webhooks, WebSocket, or message queues. No need for external tools.

### 7.7 🏷️ Multi-Tenant Namespaces with Resource Quotas
Isolated namespaces with per-namespace:
- Memory limits
- Key count limits
- Rate limiting
- Access control

### 7.8 📸 Point-in-Time Snapshots
`SNAPSHOT.CREATE`, `SNAPSHOT.LIST`, `SNAPSHOT.RESTORE` — instant snapshots for debugging, testing, rollback.

### 7.9 🔍 Full-Text Search on Values
`FT.SEARCH` — index string values and search them. No need for Elasticsearch for simple searches.

### 7.10 📈 Built-in Benchmarking Dashboard
Run benchmarks from the UI. Visualize latency distributions, throughput curves, compare configurations.

---

## 8. Scalability Architecture

### Phase 1: Single Node Performance (Current → 3 months)
```
┌─────────────────────────────────────────┐
│                FlashDB Node              │
│  ┌──────────┐  ┌──────────┐  ┌────────┐│
│  │ RESP TCP │  │ HTTP API │  │ WebSocket│
│  │ Server   │  │ Server   │  │ Server  ││
│  └────┬─────┘  └────┬─────┘  └────┬───┘│
│       └──────────────┼─────────────┘    │
│              ┌───────┴───────┐          │
│              │ Command Router│          │
│              └───────┬───────┘          │
│       ┌──────────────┼──────────────┐   │
│  ┌────┴────┐  ┌──────┴─────┐  ┌────┴──┐│
│  │ Sharded │  │  Sharded   │  │Sharded││
│  │ KV Store│  │ Type Store │  │ Index ││
│  └────┬────┘  └──────┬─────┘  └───────┘│
│       └──────────────┼──────────────┘   │
│              ┌───────┴───────┐          │
│              │  WAL Manager  │          │
│              │ (Segmented)   │          │
│              └───────┬───────┘          │
│              ┌───────┴───────┐          │
│              │  Disk Storage │          │
│              └───────────────┘          │
└─────────────────────────────────────────┘
```

### Phase 2: Replication (3-6 months)
```
┌──────────┐     ┌──────────┐     ┌──────────┐
│  Primary │────▶│ Replica 1│     │ Replica 2│
│  (R/W)   │────▶│  (Read)  │     │  (Read)  │
└──────────┘     └──────────┘     └──────────┘
      │                                 ▲
      └─────────────────────────────────┘
           Async WAL Replication
```

### Phase 3: Clustering (6-12 months)
```
┌─────────────────────────────────────────────┐
│              FlashDB Cluster                 │
│                                              │
│  ┌─────────┐  ┌─────────┐  ┌─────────┐     │
│  │ Node 1  │  │ Node 2  │  │ Node 3  │     │
│  │Slots 0-5│  │Slots 5-10│ │Slots 10-16│   │
│  └─────────┘  └─────────┘  └─────────┘     │
│       │            │            │            │
│  ┌────┴────────────┴────────────┴────┐      │
│  │       Gossip Protocol Layer       │      │
│  └───────────────────────────────────┘      │
└─────────────────────────────────────────────┘
```

---

## 9. Frontend — Professional UI/UX Redesign

### Design Philosophy
- **Minimalist** — Every pixel earns its place (inspired by Linear, Vercel, Raycast)
- **AI-Native** — Intelligence visible in every interaction
- **Real-Time** — WebSocket-driven, zero polling
- **Dark-First** — Premium dark theme with optional light mode
- **Motion** — Subtle, purposeful animations (Framer Motion)

### Technology Stack
| Layer | Technology | Rationale |
|-------|-----------|-----------|
| Framework | Next.js 15 (App Router) | Already using, upgrade to latest |
| Styling | Tailwind CSS v4 + CSS Variables | Design token system |
| Components | Radix UI primitives | Accessible, unstyled |
| Charts | Recharts or Tremor | Professional data viz |
| Animations | Framer Motion | Production-grade motion |
| State | Zustand | Lightweight, no boilerplate |
| Real-time | WebSocket (native) | Replace polling |
| Icons | Lucide React | Already using |
| Fonts | Inter + JetBrains Mono | Professional + code |

### Page Architecture
```
/                    → Dashboard (stats, live charts, quick actions)
/console             → Interactive RESP terminal
/explorer            → Key browser with type-aware editing
/monitoring          → Real-time metrics, slow log, clients
/playground          → Feature lab, benchmarks, data generator
/settings            → Server config, namespaces, ACLs
```

### UI Component Hierarchy
```
App Shell
├── Sidebar (collapsible, icon-only mode)
│   ├── Navigation items with badges
│   ├── Server status indicator
│   └── Quick command palette (⌘K)
├── Top Bar
│   ├── Breadcrumbs
│   ├── Search (global ⌘K)
│   └── Connection indicator
└── Main Content Area
    ├── Dashboard
    │   ├── KPI Cards (animated counters)
    │   ├── Operations/sec Chart (real-time streaming)
    │   ├── Memory Timeline
    │   ├── Key Distribution (treemap)
    │   └── Recent Commands (live feed)
    ├── Console
    │   ├── Terminal with syntax highlighting
    │   ├── Auto-complete suggestions
    │   ├── Command history (persistent)
    │   └── Output formatting (JSON, table, raw)
    ├── Explorer
    │   ├── Key tree (namespace-aware)
    │   ├── Type-specific editors
    │   ├── Bulk operations toolbar
    │   └── Import/Export panel
    └── Monitoring
        ├── Connected clients table
        ├── Slow query log
        ├── Memory breakdown
        └── Alert configuration
```

### Design Tokens
```css
/* Core palette — sophisticated dark theme */
--bg-primary:    #09090b;   /* zinc-950 */
--bg-secondary:  #18181b;   /* zinc-900 */
--bg-tertiary:   #27272a;   /* zinc-800 */
--border:        #3f3f46;   /* zinc-700 */
--text-primary:  #fafafa;   /* zinc-50 */
--text-secondary:#a1a1aa;   /* zinc-400 */
--accent:        #3b82f6;   /* blue-500 */
--accent-hover:  #2563eb;   /* blue-600 */
--success:       #22c55e;   /* green-500 */
--warning:       #f59e0b;   /* amber-500 */
--error:         #ef4444;   /* red-500 */
```

---

## 10. Implementation Phases

### Phase 1: Foundation (Weeks 1-3) — "Make It Correct"
- [ ] Fix sorted set WAL persistence
- [ ] Implement WAL segmentation & compaction
- [ ] Fix GC to use sampling-based expiration
- [ ] Fix MSET/MSETNX atomicity
- [ ] Fix transaction isolation
- [ ] Implement sharded locking
- [ ] Add structured logging (slog)
- [ ] Unify configuration system
- [ ] Fix TYPE command for all data types
- [ ] Fix RANDOMKEY, KEYS pattern matching, SCAN cursor

### Phase 2: Data Types (Weeks 3-5) — "Make It Complete"
- [ ] Implement Hash data type + commands
- [ ] Implement List data type + commands
- [ ] Implement Set data type + commands
- [ ] WAL persistence for all new types
- [ ] Full test coverage for new types

### Phase 3: Performance (Weeks 5-7) — "Make It Fast"
- [ ] Implement pipeline support
- [ ] Memory-mapped I/O for WAL reads
- [ ] Connection pooling
- [ ] Zero-copy optimizations
- [ ] Benchmark suite against Redis

### Phase 4: Security & Operations (Weeks 7-9) — "Make It Safe"
- [ ] TLS support for RESP connections
- [ ] ACL system (per-user permissions)
- [ ] Rate limiting per client
- [ ] Web API authentication
- [ ] Audit logging
- [ ] Slow query log

### Phase 5: Frontend Redesign (Weeks 5-10) — "Make It Beautiful"
- [ ] New app shell with sidebar navigation
- [ ] Dashboard with real-time charts (WebSocket)
- [ ] Professional console with syntax highlighting
- [ ] Type-aware key explorer
- [ ] Monitoring dashboard
- [ ] Command palette (⌘K)
- [ ] Dark/light theme system
- [ ] Responsive design
- [ ] Settings panel

### Phase 6: Advanced Features (Weeks 10-16) — "Make It Unique"
- [x] Hot key detection
- [x] Time-series data type
- [x] Change data capture (CDC)
- [x] Point-in-time snapshots
- [x] Built-in benchmarking dashboard
- [ ] AI query assistant
- [ ] Schema registry

### Phase 7: Scale (Weeks 16-24) — "Make It Big"
- [ ] Primary-replica replication
- [ ] Read scaling
- [ ] Cluster mode (hash slots)
- [ ] Cross-node pub/sub
- [ ] Cluster management UI

---

## 11. Technical Specifications

### WAL Segment Format (New)
```
Segment File: flashdb-{sequence:08d}.wal
Max Segment Size: 64 MiB
Header: Magic(4) + Version(2) + SegmentID(8) = 14 bytes
Record: same as current (CRC32 + Type + Key + Value + TTL)
```

### Sharded Lock Design
```go
const NumShards = 256

type ShardedStore struct {
    shards [NumShards]struct {
        mu   sync.RWMutex
        data map[string]*Entry
    }
}

func (s *ShardedStore) getShard(key string) int {
    h := fnv.New32a()
    h.Write([]byte(key))
    return int(h.Sum32()) % NumShards
}
```

### WebSocket Protocol (Frontend Real-time)
```json
// Server → Client
{"type": "stats", "data": {"keys": 1000, "ops": 5000, "mem": 1048576}}
{"type": "command", "data": {"cmd": "SET", "key": "foo", "latency_us": 42}}
{"type": "alert", "data": {"type": "hot_key", "key": "popular:item", "qps": 10000}}
```

---

## 12. Quality Gates

### Every PR Must Pass
- [ ] All existing tests pass
- [ ] New tests for new code (>80% coverage)
- [ ] No data races (`go test -race`)
- [ ] Benchmark comparison (no >5% regression)
- [ ] Lint clean (`golangci-lint`, `eslint`)

### Release Criteria
- [ ] All P0 issues resolved
- [ ] All data types have WAL persistence
- [ ] Frontend loads in <1s (Lighthouse >90)
- [ ] 100K ops/sec on single node
- [ ] Zero known data loss scenarios
- [ ] Security audit passed
- [ ] Documentation complete

---

*This document is the source of truth for FlashDB product development. All engineering decisions should align with this roadmap.*
