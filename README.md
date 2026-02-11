<p align="center">
  <img src="frontend/public/logo.svg" alt="FlashDB Logo" width="120" height="120">
</p>

<h1 align="center">FlashDB</h1>

<p align="center">
  <strong>⚡ A blazing-fast, Redis-compatible key-value store written in Go</strong>
</p>

<p align="center">
  <a href="#features">Features</a> •
  <a href="#quick-start">Quick Start</a> •
  <a href="#commands">Commands</a> •
  <a href="#architecture">Architecture</a> •
  <a href="#security">Security</a> •
  <a href="#documentation">Docs</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/version-1.0.0-blue.svg" alt="Version">
  <img src="https://img.shields.io/badge/go-%3E%3D1.22-00ADD8.svg" alt="Go Version">
  <img src="https://img.shields.io/badge/license-MIT-green.svg" alt="License">
  <img src="https://img.shields.io/badge/redis-compatible-red.svg" alt="Redis Compatible">
</p>

---

## ✨ Features

- **🚀 High Performance** — In-memory storage with optimized data structures
- **📦 Redis Compatible** — RESP protocol support, works with existing Redis clients
- **💾 Durable Persistence** — Write-Ahead Log (WAL) with CRC32 checksums
- **🔐 Authentication** — Optional password protection via AUTH command
- **📊 Sorted Sets** — Full ZSet implementation with 16+ commands
- **💬 Pub/Sub** — Real-time messaging with SUBSCRIBE/PUBLISH
- **🔄 Transactions** — MULTI/EXEC support with atomic operations
- **🌐 Modern Web UI** — Next.js dashboard for visual management

---

## 🚀 Quick Start

### Prerequisites

- Go 1.22 or higher
- Node.js 18+ (for web UI)

### Build & Run

```bash
# Clone the repository
git clone https://github.com/flashdb/flashdb.git
cd flashdb

# Build the server
go build -o flashdb.exe ./cmd/flashdb

# Run FlashDB
./flashdb.exe
```

### Connect with Redis CLI

```bash
redis-cli -p 6379
> SET hello world
OK
> GET hello
"world"
```

### Launch Web UI (Optional)

```bash
cd frontend
npm install
npm run dev
# Open http://localhost:3000
```

---

## ⚙️ Configuration

| Flag | Default | Description |
|------|---------|-------------|
| `-addr` | `:6379` | Server address (`host:port`) |
| `-data` | `data` | Data directory |
| `-requirepass` | ` ` | Password required for `AUTH` |
| `-maxclients` | `10000` | Maximum concurrent clients |
| `-timeout` | `0` | Client timeout in seconds (`0` = disabled) |
| `-web` | `false` | Enable legacy web UI |
| `-webaddr` | `:8080` | Legacy web UI address |
| `-version` | `false` | Print version and exit |

### Examples

```bash
# Run on custom port with authentication
./flashdb.exe -addr :6380 -requirepass mysecretpassword

# Enable legacy web interface
./flashdb.exe -web -webaddr :8080
```

---

## 📚 Commands

### String Operations

| Command | Syntax | Description |
|---------|--------|-------------|
| `SET` | `SET key value [EX seconds] [PX ms] [NX\|XX]` | Set key to value |
| `GET` | `GET key` | Get value of key |
| `MSET` | `MSET key value [key value ...]` | Set multiple keys |
| `MGET` | `MGET key [key ...]` | Get multiple values |
| `INCR` | `INCR key` | Increment by 1 |
| `DECR` | `DECR key` | Decrement by 1 |
| `INCRBY` | `INCRBY key increment` | Increment by amount |
| `DECRBY` | `DECRBY key decrement` | Decrement by amount |
| `APPEND` | `APPEND key value` | Append to string |
| `STRLEN` | `STRLEN key` | Get string length |
| `GETRANGE` | `GETRANGE key start end` | Get substring |
| `SETRANGE` | `SETRANGE key offset value` | Overwrite substring |
| `SETNX` | `SETNX key value` | Set if not exists |
| `SETEX` | `SETEX key seconds value` | Set with expiration |
| `GETSET` | `GETSET key value` | Set and return old value |

### Key Operations

| Command | Syntax | Description |
|---------|--------|-------------|
| `DEL` | `DEL key [key ...]` | Delete keys |
| `EXISTS` | `EXISTS key [key ...]` | Check if keys exist |
| `KEYS` | `KEYS pattern` | Find keys by pattern |
| `EXPIRE` | `EXPIRE key seconds` | Set TTL in seconds |
| `PEXPIRE` | `PEXPIRE key milliseconds` | Set TTL in ms |
| `TTL` | `TTL key` | Get TTL in seconds |
| `PTTL` | `PTTL key` | Get TTL in ms |
| `PERSIST` | `PERSIST key` | Remove expiration |
| `TYPE` | `TYPE key` | Get key type |
| `RENAME` | `RENAME key newkey` | Rename key |
| `RENAMENX` | `RENAMENX key newkey` | Rename if new doesn't exist |
| `RANDOMKEY` | `RANDOMKEY` | Return random key |
| `DBSIZE` | `DBSIZE` | Count keys |
| `FLUSHDB` | `FLUSHDB` | Delete all keys |
| `FLUSHALL` | `FLUSHALL` | Delete all keys |
| `SCAN` | `SCAN cursor [MATCH pattern] [COUNT count]` | Iterate keys |

### Sorted Sets (ZSet)

| Command | Syntax | Description |
|---------|--------|-------------|
| `ZADD` | `ZADD key score member [score member ...]` | Add members |
| `ZSCORE` | `ZSCORE key member` | Get member score |
| `ZRANK` | `ZRANK key member` | Get member rank (low to high) |
| `ZREVRANK` | `ZREVRANK key member` | Get member rank (high to low) |
| `ZRANGE` | `ZRANGE key start stop [WITHSCORES]` | Get range by rank |
| `ZREVRANGE` | `ZREVRANGE key start stop [WITHSCORES]` | Get reverse range |
| `ZRANGEBYSCORE` | `ZRANGEBYSCORE key min max [WITHSCORES] [LIMIT]` | Get by score range |
| `ZREVRANGEBYSCORE` | `ZREVRANGEBYSCORE key max min [WITHSCORES] [LIMIT]` | Reverse score range |
| `ZREM` | `ZREM key member [member ...]` | Remove members |
| `ZREMRANGEBYRANK` | `ZREMRANGEBYRANK key start stop` | Remove by rank range |
| `ZREMRANGEBYSCORE` | `ZREMRANGEBYSCORE key min max` | Remove by score range |
| `ZINCRBY` | `ZINCRBY key increment member` | Increment score |
| `ZCARD` | `ZCARD key` | Count members |
| `ZCOUNT` | `ZCOUNT key min max` | Count in score range |
| `ZLEXCOUNT` | `ZLEXCOUNT key min max` | Count in lex range |
| `ZPOPMIN` | `ZPOPMIN key [count]` | Pop lowest scores |
| `ZPOPMAX` | `ZPOPMAX key [count]` | Pop highest scores |

### Transactions

| Command | Syntax | Description |
|---------|--------|-------------|
| `MULTI` | `MULTI` | Start transaction |
| `EXEC` | `EXEC` | Execute transaction |
| `DISCARD` | `DISCARD` | Abort transaction |

### Pub/Sub

| Command | Syntax | Description |
|---------|--------|-------------|
| `SUBSCRIBE` | `SUBSCRIBE channel [channel ...]` | Subscribe to channels |
| `UNSUBSCRIBE` | `UNSUBSCRIBE [channel ...]` | Unsubscribe |
| `PUBLISH` | `PUBLISH channel message` | Publish message |

### Server

| Command | Syntax | Description |
|---------|--------|-------------|
| `PING` | `PING [message]` | Test connection |
| `ECHO` | `ECHO message` | Echo message |
| `INFO` | `INFO [section]` | Server information |
| `DBSIZE` | `DBSIZE` | Number of keys |
| `TIME` | `TIME` | Server time |
| `AUTH` | `AUTH password` | Authenticate |
| `COMMAND` | `COMMAND` | List commands |
| `CLIENT` | `CLIENT subcommand` | Client management |
| `CONFIG` | `CONFIG GET/SET` | Configuration |

---

## 🏗️ Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                        FlashDB Server                        │
├─────────────────────────────────────────────────────────────┤
│  ┌─────────────┐  ┌─────────────┐  ┌─────────────────────┐  │
│  │   Client    │  │   Client    │  │   Next.js Web UI    │  │
│  │  (TCP/RESP) │  │  (TCP/RESP) │  │    (HTTP/JSON)      │  │
│  └──────┬──────┘  └──────┬──────┘  └──────────┬──────────┘  │
│         │                │                    │              │
│         └────────────────┼────────────────────┘              │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   RESP Protocol Layer                  │  │
│  │            (Redis Serialization Protocol)              │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Command Router                       │  │
│  │        (Authentication • Transactions • Pub/Sub)       │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │                   Storage Engine                       │  │
│  │  ┌─────────┐  ┌─────────┐  ┌─────────┐  ┌─────────┐  │  │
│  │  │ Strings │  │  Keys   │  │  ZSets  │  │   TTL   │  │  │
│  │  └─────────┘  └─────────┘  └─────────┘  └─────────┘  │  │
│  └───────────────────────┬───────────────────────────────┘  │
│                          ▼                                   │
│  ┌───────────────────────────────────────────────────────┐  │
│  │              Write-Ahead Log (WAL)                     │  │
│  │         (CRC32 Checksums • Crash Recovery)             │  │
│  └───────────────────────────────────────────────────────┘  │
└─────────────────────────────────────────────────────────────┘
```

---

## 🔐 Security

### Authentication

FlashDB supports password authentication similar to Redis:

```bash
# Start server with password
./flashdb.exe -requirepass YourSecurePassword123

# Connect and authenticate
redis-cli -p 6379
> AUTH YourSecurePassword123
OK
> SET key value
OK
```

### Security Best Practices

- ✅ Always use authentication when exposed beyond localhost
- ✅ Bind to localhost (`-addr 127.0.0.1:6379`) when not needed externally
- ✅ Use strong passwords (16+ characters, mixed case, numbers, symbols)
- ✅ Run behind a firewall
- ✅ Monitor access logs regularly

---

## 📁 Project Structure

```
flashdb/
├── cmd/
│   └── flashdb/
│       └── main.go           # Server entry point
├── internal/
│   ├── engine/               # Storage engine
│   │   └── engine.go
│   ├── protocol/             # RESP protocol parser
│   │   └── protocol.go
│   ├── server/               # TCP server & command handlers
│   │   └── server.go
│   ├── store/                # In-memory data structures
│   │   └── store.go
│   ├── wal/                  # Write-ahead log
│   │   └── wal.go
│   └── web/                  # Legacy embedded web UI
│       └── web.go
├── frontend/                 # Next.js Web UI
│   ├── src/
│   │   ├── app/              # App router pages
│   │   ├── components/       # React components
│   │   ├── context/          # React contexts
│   │   └── lib/              # Utilities & API
│   └── public/               # Static assets
├── docs/                     # Documentation
│   ├── README.md
│   ├── COMMANDS.md
│   └── PROTOCOL.md
├── go.mod
├── go.sum
└── README.md
```

---

## 🧪 Testing

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run specific package tests
go test ./internal/engine/
go test ./internal/server/
```

---

## 📖 Documentation

Core documentation is available in the `/docs` folder:

- [**Docs Index**](docs/README.md) — Navigation entry point
- [**Commands**](docs/COMMANDS.md) — Command reference
- [**Protocol**](docs/PROTOCOL.md) — RESP protocol details

---

## 🤝 Contributing

Contributions are welcome! Please feel free to submit a Pull Request.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

---

## 📄 License

This project is licensed under the MIT License - see the [LICENSE](LICENSE) file for details.

---

<p align="center">
  Made with ❤️ by the FlashDB Team
</p>

<p align="center">
  <sub>⚡ Fast. Simple. Reliable. ⚡</sub>
</p>
