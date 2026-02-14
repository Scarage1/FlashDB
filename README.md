<p align="center">
  <img src="frontend/public/logo.svg" alt="FlashDB" width="100" height="100">
</p>

<h1 align="center">FlashDB</h1>

<p align="center">
  <strong>Lightning-fast, Redis-compatible in-memory database written in Go</strong>
</p>

<p align="center">
  <a href="https://github.com/Scarage1/FlashDB/actions/workflows/backend.yml"><img src="https://github.com/Scarage1/FlashDB/actions/workflows/backend.yml/badge.svg" alt="Backend CI"></a>
  <a href="https://github.com/Scarage1/FlashDB/actions/workflows/frontend.yml"><img src="https://github.com/Scarage1/FlashDB/actions/workflows/frontend.yml/badge.svg" alt="Frontend CI"></a>
  <a href="https://github.com/Scarage1/FlashDB/actions/workflows/deploy.yml"><img src="https://github.com/Scarage1/FlashDB/actions/workflows/deploy.yml/badge.svg" alt="Deploy"></a>
  <img src="https://img.shields.io/badge/go-%3E%3D1.22-00ADD8?logo=go" alt="Go 1.22+">
  <a href="LICENSE"><img src="https://img.shields.io/badge/license-MIT-green" alt="MIT License"></a>
</p>

---

FlashDB is a single-binary database server that speaks the Redis RESP protocol and ships with a built-in web dashboard. Drop-in compatible with `redis-cli` and any Redis client library.

## Features

- **Redis Compatible** — RESP protocol, works with any Redis client
- **Single Binary** — Go backend with embedded Next.js dashboard, zero runtime dependencies
- **Persistent** — Write-Ahead Log with CRC32 checksums and crash recovery
- **Sorted Sets** — Full ZSet implementation (ZADD, ZRANGE, ZRANGEBYSCORE, etc.)
- **Pub/Sub** — Real-time messaging with SUBSCRIBE / PUBLISH
- **Transactions** — MULTI / EXEC atomic operations
- **Web Dashboard** — Built-in admin UI at `:8080` with console, key browser, and live stats
- **Cloud Ready** — Docker image, health endpoints, env-var configuration

## Quick Start

### Docker (recommended)

```bash
docker run -d --name flashdb \
  -p 6379:6379 \
  -p 8080:8080 \
  -v flashdb-data:/home/nonroot/data \
  ghcr.io/scarage1/flashdb:latest
```

Open **http://localhost:8080** for the web dashboard.

### Docker Compose

```bash
git clone https://github.com/Scarage1/FlashDB.git && cd FlashDB
docker compose up -d
```

### From Source

```bash
# Requires Go 1.22+ and Node.js 20+
git clone https://github.com/Scarage1/FlashDB.git && cd FlashDB

# Build frontend static export
cd frontend && npm ci && npm run build && cd ..

# Copy static files into embed directory
cp -r frontend/out/* internal/web/static/

# Build and run
go build -o flashdb ./cmd/flashdb
./flashdb
```

### Connect

```bash
redis-cli -p 6379
> SET hello world
OK
> GET hello
"world"
> ZADD leaderboard 100 alice 85 bob
(integer) 2
> ZRANGE leaderboard 0 -1 WITHSCORES
1) "bob"
2) "85"
3) "alice"
4) "100"
```

## Configuration

All flags can be overridden with environment variables (useful for Docker / cloud deployments).

| Flag | Env Var | Default | Description |
|------|---------|---------|-------------|
| `-addr` | `FLASHDB_ADDR` | `:6379` | RESP server address |
| `-data` | `FLASHDB_DATA` | `data` | Persistence directory |
| `-requirepass` | `FLASHDB_PASSWORD` | | AUTH password |
| `-api-token` | `FLASHDB_API_TOKEN` | | Web API bearer token |
| `-webaddr` | `FLASHDB_WEB_ADDR` | `:8080` | Web UI & API address |
| `-maxclients` | `FLASHDB_MAXCLIENTS` | `10000` | Max concurrent clients |
| `-timeout` | `FLASHDB_TIMEOUT` | `0` | Client timeout (seconds) |
| `-loglevel` | `FLASHDB_LOG_LEVEL` | `info` | debug / info / warn / error |
| `-noweb` | `FLASHDB_NO_WEB` | `false` | Disable web UI |
| `-ratelimit` | `FLASHDB_RATELIMIT` | `0` | Max cmds/sec per client |
| `-tls-cert` | `FLASHDB_TLS_CERT` | | TLS certificate PEM |
| `-tls-key` | `FLASHDB_TLS_KEY` | | TLS private key PEM |

## Architecture

```
┌─────────────────────────────────────────────────────┐
│                   FlashDB Server                    │
│                                                     │
│  :6379 RESP          :8080 HTTP                     │
│  ┌───────────┐       ┌────────────────────────┐     │
│  │ TCP/RESP  │       │  Web UI  │  REST API   │     │
│  │ Protocol  │       │ (embed)  │  /api/v1/*  │     │
│  └─────┬─────┘       └────────┬───────────────┘     │
│        │                      │                     │
│        └──────────┬───────────┘                     │
│                   ▼                                 │
│         ┌─────────────────┐                         │
│         │  Storage Engine │                         │
│         │  Strings · Keys │                         │
│         │  ZSets · TTL    │                         │
│         └────────┬────────┘                         │
│                  ▼                                  │
│         ┌─────────────────┐                         │
│         │   WAL (CRC32)   │ ──▶ /data/flashdb.wal  │
│         └─────────────────┘                         │
└─────────────────────────────────────────────────────┘
```

## Deployment

### Azure Container Apps (free tier)

```bash
# One-time setup
az login
./deploy/azure-setup.sh

# CI/CD deploys automatically on push to master via GitHub Actions
```

See [deploy/azure-setup.sh](deploy/azure-setup.sh) for the full setup script. Required GitHub secrets:

| Secret | Description |
|--------|-------------|
| `AZURE_CREDENTIALS` | Service principal JSON (`az ad sp create-for-rbac`) |
| `AZURE_RESOURCE_GROUP` | Resource group name (e.g. `flashdb-rg`) |
| `AZURE_CONTAINER_ENV` | Container Apps environment name |

### Health Checks

| Endpoint | Purpose |
|----------|---------|
| `GET /healthz` | Liveness — always returns `200` |
| `GET /readyz` | Readiness — checks engine state |

## Documentation

- [Commands Reference](docs/COMMANDS.md) — Full list of supported commands
- [RESP Protocol](docs/PROTOCOL.md) — Wire protocol details
- [Architecture](docs/ARCHITECTURE.md) — Internal design

## Contributing

Contributions are welcome! See [CONTRIBUTING.md](CONTRIBUTING.md) for guidelines.

```bash
# Development workflow
git checkout -b feature/my-feature
go test -race ./...          # Run tests
go vet ./... && gofmt -l .   # Lint
# Submit a PR
```

## License

[MIT](LICENSE)
