# ── Stage 1: Build frontend static export ────────────────────────────────────
FROM node:20-alpine AS frontend

WORKDIR /build/frontend
COPY frontend/package.json frontend/package-lock.json ./
RUN npm ci --ignore-scripts

COPY frontend/ ./
RUN npm run build

# ── Stage 2: Build Go binary ─────────────────────────────────────────────────
FROM golang:1.22-alpine AS backend

RUN apk add --no-cache git

WORKDIR /build

# Cache Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy source
COPY cmd/ cmd/
COPY internal/ internal/

# Copy frontend static export into the embed directory
COPY --from=frontend /build/frontend/out/ internal/web/static/

# Build with version info
ARG VERSION=dev
ARG BUILD_TIME=unknown
RUN CGO_ENABLED=0 GOOS=linux go build \
    -ldflags "-s -w \
      -X github.com/flashdb/flashdb/internal/version.Version=${VERSION} \
      -X github.com/flashdb/flashdb/internal/version.BuildTime=${BUILD_TIME}" \
    -o /flashdb ./cmd/flashdb

# Create data directory for nonroot user
RUN mkdir -p /data && chown 65534:65534 /data

# ── Stage 3: Minimal runtime image ───────────────────────────────────────────
FROM gcr.io/distroless/static-debian12:nonroot

LABEL org.opencontainers.image.title="FlashDB" \
      org.opencontainers.image.description="Lightning-fast Redis-compatible in-memory database" \
      org.opencontainers.image.source="https://github.com/Scarage1/FlashDB" \
      org.opencontainers.image.licenses="MIT"

COPY --from=backend /flashdb /flashdb

# Create data directory owned by nonroot user (uid 65534)
COPY --from=backend --chown=65534:65534 /data /data

# Persistent data volume
VOLUME ["/data"]

# RESP protocol + Web UI / API
EXPOSE 6379 8080

ENTRYPOINT ["/flashdb"]
CMD ["-addr", ":6379", "-webaddr", ":8080", "-data", "/data"]
