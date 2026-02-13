# FlashDB HTTP API Reference

> Version: v1 | Base URL: `http://localhost:8080/api/v1`

---

## Authentication

When `requirepass` is configured, include the API key in headers:
```
Authorization: Bearer <api-key>
```

---

## Endpoints

### Health & Status

#### `GET /healthz`
Health check endpoint for load balancers and orchestrators.

**Response** `200 OK`
```json
{
  "status": "ok",
  "time": "2026-02-13T10:30:00Z"
}
```

#### `GET /readyz`
Readiness check — returns 503 if engine is not initialized.

**Response** `200 OK` | `503 Service Unavailable`
```json
{
  "status": "ready",
  "ready": true
}
```

#### `GET /stats`
Server statistics and metrics.

**Response** `200 OK`
```json
{
  "version": "2.0.0",
  "uptime": 86400,
  "uptime_human": "1d 0h 0m 0s",
  "keys": 12847,
  "memory_used": 25165824,
  "memory_used_mb": 24.0,
  "goroutines": 12,
  "cpus": 8,
  "total_commands": 1284700,
  "total_reads": 842100,
  "total_writes": 442600,
  "connected_clients": 3,
  "ops_per_sec": 48291
}
```

---

### Command Execution

#### `POST /execute`
Execute any FlashDB/Redis command.

**Request**
```json
{
  "command": "SET",
  "args": ["user:123", "{\"name\":\"John\"}"]
}
```

Or as a single string (auto-parsed):
```json
{
  "command": "SET user:123 \"John Doe\""
}
```

**Response** `200 OK`
```json
{
  "success": true,
  "result": "OK",
  "type": "string"
}
```

**Error Response**
```json
{
  "success": false,
  "error": "wrong number of arguments for 'SET' command"
}
```

---

### Key Operations

#### `GET /keys`
List keys with optional filtering and pagination.

**Query Parameters**
| Param | Type | Default | Description |
|-------|------|---------|-------------|
| `pattern` | string | `*` | Glob pattern to filter keys |
| `limit` | int | `100` | Maximum keys to return |
| `cursor` | int | `0` | Pagination cursor |
| `type` | string | — | Filter by type: `string`, `hash`, `list`, `set`, `zset` |

**Response** `200 OK`
```json
{
  "keys": [
    {"key": "user:1", "type": "string", "ttl": -1},
    {"key": "user:2", "type": "hash", "ttl": 3600},
    {"key": "scores", "type": "zset", "ttl": -1}
  ],
  "total": 12847,
  "cursor": 100
}
```

#### `GET /key/:key`
Get details of a specific key.

**Response** `200 OK`
```json
{
  "key": "user:123",
  "type": "string",
  "ttl": -1,
  "value": "{\"name\":\"John Doe\",\"email\":\"john@doe.com\"}",
  "size": 42,
  "encoding": "raw"
}
```

**Response** `404 Not Found`
```json
{
  "success": false,
  "error": "key not found"
}
```

#### `PUT /key/:key`
Set a key value.

**Request**
```json
{
  "value": "Hello World",
  "ttl": 3600
}
```

**Response** `200 OK`
```json
{
  "success": true
}
```

#### `DELETE /key/:key`
Delete a key.

**Response** `200 OK`
```json
{
  "success": true,
  "deleted": true
}
```

---

### Monitoring (Planned)

#### `GET /clients`
List connected clients.

**Response** `200 OK`
```json
{
  "clients": [
    {
      "id": 1,
      "addr": "127.0.0.1:8234",
      "age": 7380,
      "idle": 0,
      "commands": 12847
    }
  ]
}
```

#### `GET /slowlog`
Get slow query log entries.

**Query Parameters**
| Param | Type | Default |
|-------|------|---------|
| `limit` | int | `25` |

**Response** `200 OK`
```json
{
  "entries": [
    {
      "id": 1,
      "timestamp": 1707825600,
      "duration_us": 23000,
      "command": "KEYS *",
      "client": "127.0.0.1:8234"
    }
  ]
}
```

---

### WebSocket (Planned)

#### `WS /ws`
Real-time event stream.

**Server → Client Messages**
```json
{"type": "stats", "data": {"keys": 12847, "ops": 48291, "mem": 25165824}}
{"type": "command", "data": {"cmd": "SET", "key": "user:1", "latency_us": 42}}
{"type": "keyspace", "data": {"event": "set", "key": "user:1"}}
{"type": "alert", "data": {"type": "hot_key", "key": "popular", "qps": 10000}}
```

**Client → Server Messages**
```json
{"type": "subscribe", "channels": ["stats", "keyspace", "alerts"]}
{"type": "unsubscribe", "channels": ["keyspace"]}
```

---

## Error Codes

| HTTP Status | Error Code | Description |
|------------|------------|-------------|
| 400 | `BAD_REQUEST` | Invalid request body or parameters |
| 401 | `UNAUTHORIZED` | Missing or invalid authentication |
| 404 | `NOT_FOUND` | Key or resource not found |
| 405 | `METHOD_NOT_ALLOWED` | Wrong HTTP method |
| 429 | `RATE_LIMITED` | Too many requests |
| 500 | `INTERNAL_ERROR` | Server error |
| 503 | `NOT_READY` | Server not ready |

---

## Rate Limiting (Planned)

Default limits:
- 1000 requests/minute per IP
- 100 requests/minute for write operations

Headers:
```
X-RateLimit-Limit: 1000
X-RateLimit-Remaining: 987
X-RateLimit-Reset: 1707825660
```
