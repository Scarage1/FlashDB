# FlashDB — Issue Tracker & Technical Debt Registry

> Every issue found during the principal engineering audit.  
> Prioritized by severity. Each issue has a clear location, impact, and fix.

---

## 🔴 P0 — Critical (Data Loss / Correctness)

### ISSUE-001: Sorted Sets Not Persisted to WAL
- **File**: [internal/engine/engine.go](internal/engine/engine.go#L515)
- **Impact**: ALL sorted set data lost on restart
- **Root Cause**: ZAdd, ZRem, ZIncrBy, etc. never write WAL records
- **Fix**: Add WAL op types for all sorted set mutations
- **Tests Needed**: Recovery test that verifies sorted sets survive restart

### ISSUE-002: WAL Grows Without Bound
- **File**: [internal/wal/wal.go](internal/wal/wal.go)
- **Impact**: Disk fills up; recovery time grows linearly with operation count
- **Root Cause**: No compaction, no rotation, no snapshots
- **Fix**: Segment-based WAL (max 64MB per segment) + periodic snapshot + truncation

### ISSUE-003: GC Scans ALL Keys Every 100ms
- **File**: [internal/store/store.go](internal/store/store.go#L53)
- **Impact**: O(n) lock hold every 100ms. At 1M keys → catastrophic latency spikes
- **Root Cause**: `removeExpired()` iterates entire `data` map under write lock
- **Fix**: Redis-style sampling: pick 20 random keys, delete expired, repeat if >25% hit rate

### ISSUE-004: MSET Not Atomic
- **File**: [internal/server/server.go](internal/server/server.go) (cmdMSet)
- **Impact**: Crash during MSET leaves partial writes
- **Root Cause**: Each key SET is a separate engine call with separate WAL append
- **Fix**: Batch WAL write + batch in-memory apply under single lock

### ISSUE-005: MSETNX Race Condition
- **File**: [internal/server/server.go](internal/server/server.go) (cmdMSetNX)
- **Impact**: Another client can insert key between existence check and set
- **Root Cause**: Existence check and set are separate engine calls (lock released between them)
- **Fix**: Engine-level `MSetNX()` that holds lock for entire operation

### ISSUE-006: Transaction EXEC Not Isolated
- **File**: [internal/server/server.go](internal/server/server.go) (cmdExec)
- **Impact**: Other clients can modify data between queued commands during EXEC
- **Root Cause**: Each command in EXEC acquires/releases lock individually
- **Fix**: Hold write lock for entire EXEC block

---

## 🟠 P1 — High (Performance / Architecture)

### ISSUE-007: Single Global Mutex Bottleneck
- **File**: [internal/engine/engine.go](internal/engine/engine.go#L30)
- **Impact**: All reads block during any write; no parallel reads on different keys
- **Fix**: Sharded locking (256 shards, FNV hash for key→shard mapping)

### ISSUE-008: Triple Locking on Sorted Sets
- **Files**: engine.go mutex → store.go mutex → sortedset.go mutex
- **Impact**: Unnecessary contention, potential deadlock risk
- **Fix**: Remove SortedSet internal mutex; let Store/Engine coordinate locking

### ISSUE-009: TYPE Command Ignores Sorted Sets
- **File**: [internal/server/server.go](internal/server/server.go) (cmdType)
- **Impact**: `TYPE myzset` returns "string" instead of "zset"
- **Fix**: Check `store.ZExists(key)` first, return appropriate type

### ISSUE-010: KEYS Only Supports `*` Pattern
- **File**: [internal/server/server.go](internal/server/server.go) (cmdKeys)
- **Impact**: `KEYS user:*` returns empty array instead of matching keys
- **Root Cause**: Code returns `[]string{}` for any non-`*` pattern
- **Fix**: The SCAN command already has glob→regex conversion — reuse it for KEYS

### ISSUE-011: RANDOMKEY Is Not Random
- **File**: [internal/server/server.go](internal/server/server.go) (cmdRandomKey)
- **Impact**: Always returns `keys[0]`, not random
- **Fix**: Use `math/rand` to pick random index

### ISSUE-012: SCAN Cursor Breaks on Data Changes
- **File**: [internal/server/server.go](internal/server/server.go) (cmdScan)
- **Impact**: Adding/removing keys between SCAN calls causes missed or duplicate keys
- **Root Cause**: Uses array index as cursor; array changes between calls
- **Fix**: Implement Redis-style reverse bit cursor over hash table slots

### ISSUE-013: Config Package Unused
- **File**: [internal/config/config.go](internal/config/config.go)
- **Impact**: Configuration is scattered across flag parsing in main.go
- **Fix**: Wire config package into main.go; support file + env + flags

### ISSUE-014: Version Hardcoded in 3 Places
- **Files**: cmd/flashdb/main.go, internal/server/server.go, internal/web/web.go
- **Impact**: Version drift between components
- **Fix**: Single `internal/version/version.go` package, set via `-ldflags`

### ISSUE-015: No Structured Logging
- **All files using `log.Printf`**
- **Impact**: No log levels, no JSON output, no request correlation IDs
- **Fix**: Migrate to `log/slog` (Go 1.21+ stdlib)

---

## 🟡 P2 — Medium (Missing Features / UX)

### ISSUE-016: No Hash Data Type
- **Impact**: Can't use HSET, HGET, HGETALL — fundamental Redis feature
- **Fix**: Implement hash.go in store package + engine methods + server commands

### ISSUE-017: No List Data Type
- **Impact**: Can't use LPUSH, RPUSH, LPOP, RPOP, LRANGE
- **Fix**: Implement list.go with doubly-linked list + server commands

### ISSUE-018: No Set Data Type
- **Impact**: Can't use SADD, SREM, SMEMBERS, SINTER, SUNION
- **Fix**: Implement set.go with map-based set + server commands

### ISSUE-019: No TLS Support
- **Impact**: All data transmitted in plaintext
- **Fix**: Add `tls.Listen` wrapper with certificate configuration

### ISSUE-020: No ACL System
- **Impact**: Only single password; can't restrict commands per user
- **Fix**: Implement ACL similar to Redis 6+ ACL system

### ISSUE-021: No Pipeline Support
- **Impact**: Clients can't batch commands; latency overhead per command
- **Fix**: Buffer multiple commands before flush; process batch atomically

### ISSUE-022: CORS Wide Open
- **File**: [internal/web/web.go](internal/web/web.go) (corsMiddleware)
- **Impact**: `Access-Control-Allow-Origin: *` — any website can access the API
- **Fix**: Configurable allowed origins; default to same-origin only

### ISSUE-023: No Web API Authentication
- **File**: [internal/web/web.go](internal/web/web.go)
- **Impact**: Anyone can execute commands, delete all data via HTTP
- **Fix**: API key or token-based auth; respect `requirepass` config

---

## 📊 Summary

| Priority | Count | Status |
|----------|-------|--------|
| 🔴 P0 Critical | 6 | Not started |
| 🟠 P1 High | 9 | Not started |
| 🟡 P2 Medium | 8 | Not started |
| **Total** | **23** | — |

---

*Track progress by marking issues as `[x]` when resolved. Every fix requires tests.*
