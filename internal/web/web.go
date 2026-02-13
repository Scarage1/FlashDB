// Package web provides the HTTP web interface for FlashDB.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/flashdb/flashdb/internal/store"
	"github.com/flashdb/flashdb/internal/version"
)

//go:embed all:static
var staticFiles embed.FS

// Server represents the web server for FlashDB admin interface.
type Server struct {
	addr      string
	engine    *engine.Engine
	server    *http.Server
	startTime time.Time
	mu        sync.RWMutex
	apiToken  string // shared secret for API auth (empty = no auth)
}

const apiVersionPath = "/api/v1"

// New creates a new web server.
func New(addr string, engine *engine.Engine) *Server {
	return NewWithToken(addr, engine, "")
}

// NewWithToken creates a new web server with an optional API auth token.
func NewWithToken(addr string, engine *engine.Engine, token string) *Server {
	return &Server{
		addr:      addr,
		engine:    engine,
		startTime: time.Now(),
		apiToken:  token,
	}
}

// CommandRequest represents a command execution request.
type CommandRequest struct {
	Command string   `json:"command"`
	Args    []string `json:"args,omitempty"`
}

// CommandResponse represents a command execution response.
type CommandResponse struct {
	Success bool        `json:"success"`
	Result  interface{} `json:"result,omitempty"`
	Error   string      `json:"error,omitempty"`
	Type    string      `json:"type,omitempty"`
}

// StatsResponse represents server statistics.
type StatsResponse struct {
	Version       string  `json:"version"`
	Uptime        int64   `json:"uptime"`
	UptimeHuman   string  `json:"uptime_human"`
	Keys          int     `json:"keys"`
	MemoryUsed    uint64  `json:"memory_used"`
	MemoryUsedMB  float64 `json:"memory_used_mb"`
	GoRoutines    int     `json:"goroutines"`
	CPUs          int     `json:"cpus"`
	TotalCommands int64   `json:"total_commands"`
}

// KeyInfo represents information about a key.
type KeyInfo struct {
	Key   string `json:"key"`
	Type  string `json:"type"`
	TTL   int64  `json:"ttl"`
	Value string `json:"value,omitempty"`
}

// Start starts the web server.
func (s *Server) Start(ctx context.Context) error {
	mux := s.routes()

	// Serve static files
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		return fmt.Errorf("failed to create static file system: %w", err)
	}

	// SPA-aware file server: serves static files if they exist,
	// otherwise falls back to the page-specific HTML (Next.js static export)
	// or index.html for client-side routing.
	serveFile := func(w http.ResponseWriter, r *http.Request, name string) {
		f, err := staticFS.Open(name)
		if err != nil {
			http.NotFound(w, r)
			return
		}
		defer f.Close()
		stat, err := f.Stat()
		if err != nil || stat.IsDir() {
			http.NotFound(w, r)
			return
		}
		http.ServeContent(w, r, name, stat.ModTime(), f.(io.ReadSeeker))
	}

	fileServer := http.FileServer(http.FS(staticFS))
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path

		// Root path → serve index.html directly (avoid FileServer 301 redirect)
		if path == "/" {
			serveFile(w, r, "index.html")
			return
		}

		// Try the exact file first (CSS, JS, images, etc.)
		trimmed := strings.TrimPrefix(path, "/")
		if f, err := staticFS.Open(trimmed); err == nil {
			stat, _ := f.Stat()
			f.Close()
			if stat != nil && !stat.IsDir() {
				fileServer.ServeHTTP(w, r)
				return
			}
		}

		// For page routes, Next.js exports e.g. /console.html for /console
		// Try serving the corresponding .html file
		htmlPath := trimmed + ".html"
		if f, err := staticFS.Open(htmlPath); err == nil {
			f.Close()
			serveFile(w, r, htmlPath)
			return
		}

		// Fallback to index.html for any unmatched route
		serveFile(w, r, "index.html")
	})

	s.server = &http.Server{
		Addr:              s.addr,
		Handler:           s.authMiddleware(corsMiddleware(mux)),
		ReadHeaderTimeout: 10 * time.Second,
		ReadTimeout:       30 * time.Second,
		WriteTimeout:      60 * time.Second,
		IdleTimeout:       120 * time.Second,
		MaxHeaderBytes:    1 << 20, // 1 MiB
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		s.server.Shutdown(shutdownCtx)
	}()

	if err := s.server.ListenAndServe(); err != http.ErrServerClosed {
		return err
	}
	return nil
}

func (s *Server) routes() *http.ServeMux {
	mux := http.NewServeMux()

	// Legacy API routes (kept for backward compatibility)
	mux.HandleFunc("/api/execute", s.handleExecute)
	mux.HandleFunc("/api/stats", s.handleStats)
	mux.HandleFunc("/api/keys", s.handleKeys)
	mux.HandleFunc("/api/key/", s.handleKey)

	// Versioned API routes (contract-first API surface)
	mux.HandleFunc(apiVersionPath+"/execute", s.handleExecute)
	mux.HandleFunc(apiVersionPath+"/stats", s.handleStats)
	mux.HandleFunc(apiVersionPath+"/keys", s.handleKeys)
	mux.HandleFunc(apiVersionPath+"/key/", s.handleKeyV1)

	// Health endpoints
	mux.HandleFunc("/healthz", s.handleHealth)
	mux.HandleFunc("/readyz", s.handleReady)
	mux.HandleFunc(apiVersionPath+"/healthz", s.handleHealth)
	mux.HandleFunc(apiVersionPath+"/readyz", s.handleReady)

	// Phase 6 API endpoints
	mux.HandleFunc(apiVersionPath+"/hotkeys", s.handleHotKeys)
	mux.HandleFunc(apiVersionPath+"/timeseries/", s.handleTimeSeries)
	mux.HandleFunc(apiVersionPath+"/cdc", s.handleCDC)
	mux.HandleFunc(apiVersionPath+"/cdc/stream", s.handleCDCStream)
	mux.HandleFunc(apiVersionPath+"/snapshots", s.handleSnapshots)
	mux.HandleFunc(apiVersionPath+"/benchmark", s.handleBenchmark)

	return mux
}

// corsMiddleware adds CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}

		next.ServeHTTP(w, r)
	})
}

// authMiddleware validates the API token for non-public endpoints.
// Health endpoints (/healthz, /readyz) and static assets are exempt.
func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// No token configured = open access.
		if s.apiToken == "" {
			next.ServeHTTP(w, r)
			return
		}

		// Allow health probes and static assets without auth.
		p := r.URL.Path
		if p == "/healthz" || p == "/readyz" ||
			p == apiVersionPath+"/healthz" || p == apiVersionPath+"/readyz" ||
			(!strings.HasPrefix(p, "/api")) {
			next.ServeHTTP(w, r)
			return
		}

		// Check Authorization: Bearer <token>.
		auth := r.Header.Get("Authorization")
		if !strings.HasPrefix(auth, "Bearer ") || auth[7:] != s.apiToken {
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusUnauthorized)
			w.Write([]byte(`{"error":"unauthorized","message":"valid Bearer token required"}`))
			return
		}

		next.ServeHTTP(w, r)
	})
}

// handleExecute handles command execution.
func (s *Server) handleExecute(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var req CommandRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, CommandResponse{Success: false, Error: "Invalid request"})
		return
	}

	// Parse command string if args not provided
	cmd := strings.ToUpper(strings.TrimSpace(req.Command))
	args := req.Args

	if len(args) == 0 && strings.Contains(req.Command, " ") {
		parts := parseCommand(req.Command)
		if len(parts) > 0 {
			cmd = strings.ToUpper(parts[0])
			args = parts[1:]
		}
	}

	result, err := s.executeCommand(cmd, args)
	if err != nil {
		writeJSON(w, CommandResponse{Success: false, Error: err.Error()})
		return
	}

	writeJSON(w, CommandResponse{Success: true, Result: result, Type: fmt.Sprintf("%T", result)})
}

// executeCommand executes a FlashDB command.
func (s *Server) executeCommand(cmd string, args []string) (interface{}, error) {
	switch cmd {
	case "PING":
		if len(args) > 0 {
			return args[0], nil
		}
		return "PONG", nil

	case "SET":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SET' command")
		}
		// Handle TTL options
		ttl := time.Duration(0)
		for i := 2; i < len(args); i++ {
			opt := strings.ToUpper(args[i])
			if opt == "EX" && i+1 < len(args) {
				secs, _ := strconv.ParseInt(args[i+1], 10, 64)
				ttl = time.Duration(secs) * time.Second
				i++
			} else if opt == "PX" && i+1 < len(args) {
				ms, _ := strconv.ParseInt(args[i+1], 10, 64)
				ttl = time.Duration(ms) * time.Millisecond
				i++
			}
		}
		if ttl > 0 {
			return "OK", s.engine.SetWithTTL(args[0], []byte(args[1]), ttl)
		}
		return "OK", s.engine.Set(args[0], []byte(args[1]))

	case "GET":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'GET' command")
		}
		val, exists := s.engine.Get(args[0])
		if !exists {
			return nil, nil
		}
		return string(val), nil

	case "DEL":
		if len(args) < 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'DEL' command")
		}
		count := 0
		for _, key := range args {
			if deleted, _ := s.engine.Delete(key); deleted {
				count++
			}
		}
		return count, nil

	case "EXISTS":
		if len(args) < 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'EXISTS' command")
		}
		count := 0
		for _, key := range args {
			if s.engine.Exists(key) {
				count++
			}
		}
		return count, nil

	case "KEYS":
		pattern := "*"
		if len(args) > 0 {
			pattern = args[0]
		}
		allKeys := s.engine.Keys()
		return filterKeys(allKeys, pattern), nil

	case "TTL":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'TTL' command")
		}
		return s.engine.TTL(args[0]), nil

	case "EXPIRE":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'EXPIRE' command")
		}
		secs, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("invalid expire time")
		}
		ok, err := s.engine.Expire(args[0], time.Duration(secs)*time.Second)
		if err != nil {
			return nil, err
		}
		if ok {
			return 1, nil
		}
		return 0, nil

	case "INCR":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'INCR' command")
		}
		return s.engine.IncrBy(args[0], 1)

	case "DECR":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'DECR' command")
		}
		return s.engine.IncrBy(args[0], -1)

	case "INCRBY":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'INCRBY' command")
		}
		delta, err := strconv.ParseInt(args[1], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("value is not an integer")
		}
		return s.engine.IncrBy(args[0], delta)

	case "ZADD":
		if len(args) < 3 || (len(args)-1)%2 != 0 {
			return nil, fmt.Errorf("wrong number of arguments for 'ZADD' command")
		}
		key := args[0]
		members := make([]store.ScoredMember, 0, (len(args)-1)/2)
		for i := 1; i < len(args); i += 2 {
			score, err := strconv.ParseFloat(args[i], 64)
			if err != nil {
				return nil, fmt.Errorf("value is not a valid float")
			}
			members = append(members, store.ScoredMember{
				Score:  score,
				Member: args[i+1],
			})
		}
		return s.engine.ZAdd(key, members...)

	case "ZSCORE":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'ZSCORE' command")
		}
		score, exists := s.engine.ZScore(args[0], args[1])
		if !exists {
			return nil, nil
		}
		return strconv.FormatFloat(score, 'f', -1, 64), nil

	case "ZRANGE":
		if len(args) < 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'ZRANGE' command")
		}
		start, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		stop, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}

		withScores := len(args) > 3 && strings.EqualFold(args[3], "WITHSCORES")
		members := s.engine.ZRange(args[0], start, stop, withScores)
		if withScores {
			result := make([]string, 0, len(members)*2)
			for _, member := range members {
				result = append(result, member.Member, strconv.FormatFloat(member.Score, 'f', -1, 64))
			}
			return result, nil
		}

		result := make([]string, 0, len(members))
		for _, member := range members {
			result = append(result, member.Member)
		}
		return result, nil

	case "APPEND":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'APPEND' command")
		}
		return s.engine.Append(args[0], []byte(args[1]))

	case "STRLEN":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'STRLEN' command")
		}
		return s.engine.StrLen(args[0]), nil

	case "DBSIZE":
		return s.engine.Size(), nil

	case "FLUSHDB", "FLUSHALL":
		return "OK", s.engine.Clear()

	case "TYPE":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'TYPE' command")
		}
		return s.engine.KeyType(args[0]), nil

	// ── Hash commands ──────────────────────────────────────────────
	case "HSET":
		if len(args) < 3 || len(args)%2 == 0 {
			return nil, fmt.Errorf("wrong number of arguments for 'HSET' command")
		}
		fields := make([]store.HashFieldValue, 0, (len(args)-1)/2)
		for i := 1; i < len(args); i += 2 {
			fields = append(fields, store.HashFieldValue{Field: args[i], Value: []byte(args[i+1])})
		}
		return s.engine.HSet(args[0], fields...)

	case "HMSET":
		if len(args) < 3 || len(args)%2 == 0 {
			return nil, fmt.Errorf("wrong number of arguments for 'HMSET' command")
		}
		fields := make([]store.HashFieldValue, 0, (len(args)-1)/2)
		for i := 1; i < len(args); i += 2 {
			fields = append(fields, store.HashFieldValue{Field: args[i], Value: []byte(args[i+1])})
		}
		if _, err := s.engine.HSet(args[0], fields...); err != nil {
			return nil, err
		}
		return "OK", nil

	case "HGET":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'HGET' command")
		}
		val, ok := s.engine.HGet(args[0], args[1])
		if !ok {
			return nil, nil
		}
		return string(val), nil

	case "HMGET":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'HMGET' command")
		}
		result := make([]interface{}, len(args)-1)
		for i := 1; i < len(args); i++ {
			val, ok := s.engine.HGet(args[0], args[i])
			if ok {
				result[i-1] = string(val)
			}
		}
		return result, nil

	case "HDEL":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'HDEL' command")
		}
		return s.engine.HDel(args[0], args[1:]...)

	case "HEXISTS":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'HEXISTS' command")
		}
		if s.engine.HExists(args[0], args[1]) {
			return 1, nil
		}
		return 0, nil

	case "HLEN":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'HLEN' command")
		}
		return s.engine.HLen(args[0]), nil

	case "HGETALL":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'HGETALL' command")
		}
		pairs := s.engine.HGetAll(args[0])
		result := make([]string, 0, len(pairs)*2)
		for _, p := range pairs {
			result = append(result, p.Field, string(p.Value))
		}
		return result, nil

	case "HKEYS":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'HKEYS' command")
		}
		return s.engine.HKeys(args[0]), nil

	case "HVALS":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'HVALS' command")
		}
		vals := s.engine.HVals(args[0])
		result := make([]string, len(vals))
		for i, v := range vals {
			result[i] = string(v)
		}
		return result, nil

	case "HINCRBY":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'HINCRBY' command")
		}
		delta, err := strconv.ParseInt(args[2], 10, 64)
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		return s.engine.HIncrBy(args[0], args[1], delta)

	case "HINCRBYFLOAT":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'HINCRBYFLOAT' command")
		}
		delta, err := strconv.ParseFloat(args[2], 64)
		if err != nil {
			return nil, fmt.Errorf("value is not a valid float")
		}
		result, err := s.engine.HIncrByFloat(args[0], args[1], delta)
		if err != nil {
			return nil, err
		}
		return strconv.FormatFloat(result, 'f', -1, 64), nil

	case "HSETNX":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'HSETNX' command")
		}
		ok, err := s.engine.HSetNX(args[0], args[1], []byte(args[2]))
		if err != nil {
			return nil, err
		}
		if ok {
			return 1, nil
		}
		return 0, nil

	// ── List commands ──────────────────────────────────────────────
	case "LPUSH":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'LPUSH' command")
		}
		values := make([][]byte, len(args)-1)
		for i := 1; i < len(args); i++ {
			values[i-1] = []byte(args[i])
		}
		return s.engine.LPush(args[0], values...)

	case "RPUSH":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'RPUSH' command")
		}
		values := make([][]byte, len(args)-1)
		for i := 1; i < len(args); i++ {
			values[i-1] = []byte(args[i])
		}
		return s.engine.RPush(args[0], values...)

	case "LPOP":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'LPOP' command")
		}
		val, ok, err := s.engine.LPop(args[0])
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return string(val), nil

	case "RPOP":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'RPOP' command")
		}
		val, ok, err := s.engine.RPop(args[0])
		if err != nil {
			return nil, err
		}
		if !ok {
			return nil, nil
		}
		return string(val), nil

	case "LLEN":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'LLEN' command")
		}
		return s.engine.LLen(args[0]), nil

	case "LINDEX":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'LINDEX' command")
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		val, ok := s.engine.LIndex(args[0], idx)
		if !ok {
			return nil, nil
		}
		return string(val), nil

	case "LSET":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'LSET' command")
		}
		idx, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		if err := s.engine.LSet(args[0], idx, []byte(args[2])); err != nil {
			return nil, err
		}
		return "OK", nil

	case "LRANGE":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'LRANGE' command")
		}
		start, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		stop, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		items := s.engine.LRange(args[0], start, stop)
		result := make([]string, len(items))
		for i, v := range items {
			result[i] = string(v)
		}
		return result, nil

	case "LINSERT":
		if len(args) != 4 {
			return nil, fmt.Errorf("wrong number of arguments for 'LINSERT' command")
		}
		before := strings.EqualFold(args[1], "BEFORE")
		if !before && !strings.EqualFold(args[1], "AFTER") {
			return nil, fmt.Errorf("syntax error")
		}
		return s.engine.LInsert(args[0], before, []byte(args[2]), []byte(args[3]))

	case "LREM":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'LREM' command")
		}
		count, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		return s.engine.LRem(args[0], count, []byte(args[2]))

	case "LTRIM":
		if len(args) != 3 {
			return nil, fmt.Errorf("wrong number of arguments for 'LTRIM' command")
		}
		start, err := strconv.Atoi(args[1])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		stop, err := strconv.Atoi(args[2])
		if err != nil {
			return nil, fmt.Errorf("value is not an integer or out of range")
		}
		if err := s.engine.LTrim(args[0], start, stop); err != nil {
			return nil, err
		}
		return "OK", nil

	// ── Set commands ───────────────────────────────────────────────
	case "SADD":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SADD' command")
		}
		return s.engine.SAdd(args[0], args[1:]...)

	case "SREM":
		if len(args) < 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SREM' command")
		}
		return s.engine.SRem(args[0], args[1:]...)

	case "SISMEMBER":
		if len(args) != 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SISMEMBER' command")
		}
		if s.engine.SIsMember(args[0], args[1]) {
			return 1, nil
		}
		return 0, nil

	case "SCARD":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'SCARD' command")
		}
		return s.engine.SCard(args[0]), nil

	case "SMEMBERS":
		if len(args) != 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'SMEMBERS' command")
		}
		return s.engine.SMembers(args[0]), nil

	case "SRANDMEMBER":
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SRANDMEMBER' command")
		}
		count := 1
		if len(args) == 2 {
			var err error
			count, err = strconv.Atoi(args[1])
			if err != nil {
				return nil, fmt.Errorf("value is not an integer or out of range")
			}
		}
		members := s.engine.SRandMember(args[0], count)
		if len(args) == 1 {
			if len(members) == 0 {
				return nil, nil
			}
			return members[0], nil
		}
		return members, nil

	case "SPOP":
		if len(args) < 1 || len(args) > 2 {
			return nil, fmt.Errorf("wrong number of arguments for 'SPOP' command")
		}
		count := 1
		if len(args) == 2 {
			var err error
			count, err = strconv.Atoi(args[1])
			if err != nil {
				return nil, fmt.Errorf("value is not an integer or out of range")
			}
		}
		members, err := s.engine.SPop(args[0], count)
		if err != nil {
			return nil, err
		}
		if len(args) == 1 {
			if len(members) == 0 {
				return nil, nil
			}
			return members[0], nil
		}
		return members, nil

	case "SINTER":
		if len(args) < 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'SINTER' command")
		}
		return s.engine.SInter(args...), nil

	case "SUNION":
		if len(args) < 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'SUNION' command")
		}
		return s.engine.SUnion(args...), nil

	case "SDIFF":
		if len(args) < 1 {
			return nil, fmt.Errorf("wrong number of arguments for 'SDIFF' command")
		}
		return s.engine.SDiff(args...), nil

	case "INFO":
		stats := s.engine.GetStats()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		info := fmt.Sprintf("# Server\nflashdb_version:%s\nuptime_in_seconds:%d\n\n# Keyspace\nkeys:%d\ntotal_reads:%d\ntotal_writes:%d\n\n# Memory\nused_memory:%d\n",
			version.Version,
			int64(time.Since(s.startTime).Seconds()),
			stats.KeysCount,
			stats.TotalReads,
			stats.TotalWrites,
			m.Alloc)
		return info, nil

	default:
		return nil, fmt.Errorf("unknown command '%s'", cmd)
	}
}

// handleStats returns server statistics.
func (s *Server) handleStats(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	uptime := time.Since(s.startTime)
	stats := s.engine.GetStats()

	resp := StatsResponse{
		Version:       version.Version,
		Uptime:        int64(uptime.Seconds()),
		UptimeHuman:   formatDuration(uptime),
		Keys:          s.engine.Size(),
		MemoryUsed:    m.Alloc,
		MemoryUsedMB:  float64(m.Alloc) / 1024 / 1024,
		GoRoutines:    runtime.NumGoroutine(),
		CPUs:          runtime.NumCPU(),
		TotalCommands: stats.TotalCommands,
	}

	writeJSON(w, resp)
}

// handleKeys returns all keys with optional pattern filtering.
func (s *Server) handleKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	pattern := r.URL.Query().Get("pattern")
	if pattern == "" {
		pattern = "*"
	}

	limit := 100
	if l := r.URL.Query().Get("limit"); l != "" {
		if n, err := strconv.Atoi(l); err == nil && n > 0 {
			limit = n
		}
	}

	allKeys := s.engine.Keys()
	keys := filterKeys(allKeys, pattern)
	if len(keys) > limit {
		keys = keys[:limit]
	}

	keyInfos := make([]KeyInfo, len(keys))
	for i, key := range keys {
		ttl := s.engine.TTL(key)
		keyInfos[i] = KeyInfo{
			Key:  key,
			Type: s.engine.KeyType(key),
			TTL:  ttl,
		}
	}

	writeJSON(w, map[string]interface{}{
		"keys":  keyInfos,
		"total": s.engine.Size(),
	})
}

// handleKey handles individual key operations.
func (s *Server) handleKey(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, "/api/key/")
	if key == "" {
		http.Error(w, "Key required", http.StatusBadRequest)
		return
	}

	switch r.Method {
	case "GET":
		keyType := s.engine.KeyType(key)
		if keyType == "none" {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		ttl := s.engine.TTL(key)
		var value string
		switch keyType {
		case "string":
			if val, ok := s.engine.Get(key); ok {
				value = string(val)
			}
		case "hash":
			pairs := s.engine.HGetAll(key)
			m := make(map[string]string, len(pairs))
			for _, p := range pairs {
				m[p.Field] = string(p.Value)
			}
			b, _ := json.Marshal(m)
			value = string(b)
		case "list":
			items := s.engine.LRange(key, 0, -1)
			strs := make([]string, len(items))
			for i, v := range items {
				strs[i] = string(v)
			}
			b, _ := json.Marshal(strs)
			value = string(b)
		case "set":
			members := s.engine.SMembers(key)
			b, _ := json.Marshal(members)
			value = string(b)
		case "zset":
			members := s.engine.ZRange(key, 0, -1, true)
			m := make(map[string]float64, len(members))
			for _, sm := range members {
				m[sm.Member] = sm.Score
			}
			b, _ := json.Marshal(m)
			value = string(b)
		}
		writeJSON(w, KeyInfo{
			Key:   key,
			Type:  keyType,
			TTL:   ttl,
			Value: value,
		})

	case "DELETE":
		if _, err := s.engine.Delete(key); err != nil {
			writeJSON(w, map[string]interface{}{"success": false, "error": err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"success": true})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleKeyV1 handles versioned individual key operations.
func (s *Server) handleKeyV1(w http.ResponseWriter, r *http.Request) {
	key := strings.TrimPrefix(r.URL.Path, apiVersionPath+"/key/")
	if key == "" {
		http.Error(w, "Key required", http.StatusBadRequest)
		return
	}

	// Rewrite to legacy path and delegate to the existing handler.
	r.URL.Path = "/api/key/" + key
	s.handleKey(w, r)
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	writeJSON(w, map[string]interface{}{
		"status": "ok",
		"time":   time.Now().UTC().Format(time.RFC3339),
	})
}

func (s *Server) handleReady(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ready := s.engine != nil
	statusCode := http.StatusOK
	status := "ready"
	if !ready {
		statusCode = http.StatusServiceUnavailable
		status = "not_ready"
	}
	writeJSONWithStatus(w, statusCode, map[string]interface{}{
		"status": status,
		"ready":  ready,
	})
}

// filterKeys filters keys by glob pattern.
func filterKeys(keys []string, pattern string) []string {
	if pattern == "*" {
		return keys
	}

	// Convert glob to regex
	regexStr := "^"
	for _, c := range pattern {
		switch c {
		case '*':
			regexStr += ".*"
		case '?':
			regexStr += "."
		case '.', '+', '^', '$', '[', ']', '(', ')', '{', '}', '|', '\\':
			regexStr += "\\" + string(c)
		default:
			regexStr += string(c)
		}
	}
	regexStr += "$"

	re, err := regexp.Compile(regexStr)
	if err != nil {
		return keys
	}

	var result []string
	for _, key := range keys {
		if re.MatchString(key) {
			result = append(result, key)
		}
	}
	return result
}

// ========================
// Phase 6 API Handlers
// ========================

// handleHotKeys returns the top N most frequently accessed keys.
func (s *Server) handleHotKeys(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	n := 20
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	entries := s.engine.HotKeys(n)
	type hkEntry struct {
		Key   string `json:"key"`
		Count int64  `json:"count"`
	}
	result := make([]hkEntry, len(entries))
	for i, e := range entries {
		result[i] = hkEntry{Key: e.Key, Count: e.Count}
	}
	writeJSON(w, map[string]interface{}{"hotkeys": result})
}

// handleTimeSeries handles TS API calls.
func (s *Server) handleTimeSeries(w http.ResponseWriter, r *http.Request) {
	// Extract key from URL: /api/v1/timeseries/{key}
	path := strings.TrimPrefix(r.URL.Path, apiVersionPath+"/timeseries/")
	key := strings.TrimRight(path, "/")

	switch r.Method {
	case "POST":
		// Add data point
		var req struct {
			Timestamp int64   `json:"timestamp"`
			Value     float64 `json:"value"`
			Retention int64   `json:"retention_ms"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSONWithStatus(w, http.StatusBadRequest, CommandResponse{Error: "invalid JSON"})
			return
		}
		retention := time.Duration(req.Retention) * time.Millisecond
		ts, err := s.engine.TSAdd(key, req.Timestamp, req.Value, retention)
		if err != nil {
			writeJSONWithStatus(w, http.StatusInternalServerError, CommandResponse{Error: err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"timestamp": ts})

	case "GET":
		// Range query or latest
		fromStr := r.URL.Query().Get("from")
		toStr := r.URL.Query().Get("to")
		if fromStr != "" && toStr != "" {
			from, _ := strconv.ParseInt(fromStr, 10, 64)
			to, _ := strconv.ParseInt(toStr, 10, 64)
			points, err := s.engine.TSRange(key, from, to)
			if err != nil {
				writeJSONWithStatus(w, http.StatusNotFound, CommandResponse{Error: err.Error()})
				return
			}
			writeJSON(w, map[string]interface{}{"points": points})
		} else if r.URL.Query().Get("info") == "true" {
			info, err := s.engine.TSInfo(key)
			if err != nil {
				writeJSONWithStatus(w, http.StatusNotFound, CommandResponse{Error: err.Error()})
				return
			}
			writeJSON(w, info)
		} else {
			dp, ok := s.engine.TSGet(key)
			if !ok {
				writeJSONWithStatus(w, http.StatusNotFound, CommandResponse{Error: "key not found"})
				return
			}
			writeJSON(w, dp)
		}

	case "DELETE":
		ok, err := s.engine.TSDel(key)
		if err != nil {
			writeJSONWithStatus(w, http.StatusInternalServerError, CommandResponse{Error: err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"deleted": ok})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleCDC returns recent CDC events.
func (s *Server) handleCDC(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	afterStr := r.URL.Query().Get("after")
	if afterStr != "" {
		afterID, err := strconv.ParseUint(afterStr, 10, 64)
		if err != nil {
			writeJSONWithStatus(w, http.StatusBadRequest, CommandResponse{Error: "invalid after id"})
			return
		}
		events := s.engine.CDCSince(afterID)
		writeJSON(w, map[string]interface{}{"events": events, "stats": s.engine.CDCStats()})
		return
	}

	n := 50
	if q := r.URL.Query().Get("n"); q != "" {
		if v, err := strconv.Atoi(q); err == nil && v > 0 {
			n = v
		}
	}
	events := s.engine.CDCLatest(n)
	writeJSON(w, map[string]interface{}{"events": events, "stats": s.engine.CDCStats()})
}

// handleCDCStream provides Server-Sent Events for real-time CDC.
func (s *Server) handleCDCStream(w http.ResponseWriter, r *http.Request) {
	if r.Method != "GET" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "Streaming not supported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")

	subID, ch := s.engine.CDCSubscribe(256)
	defer s.engine.CDCUnsubscribe(subID)

	ctx := r.Context()
	for {
		select {
		case <-ctx.Done():
			return
		case ev, ok := <-ch:
			if !ok {
				return
			}
			fmt.Fprintf(w, "id: %d\ndata: %s\n\n", ev.ID, ev.JSON())
			flusher.Flush()
		}
	}
}

// handleSnapshots handles snapshot CRUD.
func (s *Server) handleSnapshots(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case "GET":
		metas, err := s.engine.SnapshotList()
		if err != nil {
			writeJSONWithStatus(w, http.StatusInternalServerError, CommandResponse{Error: err.Error()})
			return
		}
		writeJSON(w, map[string]interface{}{"snapshots": metas})

	case "POST":
		var req struct {
			ID string `json:"id"`
		}
		json.NewDecoder(r.Body).Decode(&req)
		meta, err := s.engine.SnapshotCreate(req.ID)
		if err != nil {
			writeJSONWithStatus(w, http.StatusInternalServerError, CommandResponse{Error: err.Error()})
			return
		}
		w.WriteHeader(http.StatusCreated)
		writeJSON(w, meta)

	case "PUT":
		var req struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.ID == "" {
			writeJSONWithStatus(w, http.StatusBadRequest, CommandResponse{Error: "id required"})
			return
		}
		if err := s.engine.SnapshotRestore(req.ID); err != nil {
			writeJSONWithStatus(w, http.StatusInternalServerError, CommandResponse{Error: err.Error()})
			return
		}
		writeJSON(w, map[string]string{"status": "restored", "id": req.ID})

	case "DELETE":
		id := r.URL.Query().Get("id")
		if id == "" {
			writeJSONWithStatus(w, http.StatusBadRequest, CommandResponse{Error: "id required"})
			return
		}
		if err := s.engine.SnapshotDelete(id); err != nil {
			writeJSONWithStatus(w, http.StatusNotFound, CommandResponse{Error: err.Error()})
			return
		}
		writeJSON(w, map[string]string{"status": "deleted", "id": id})

	default:
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
	}
}

// handleBenchmark runs an inline benchmark.
func (s *Server) handleBenchmark(w http.ResponseWriter, r *http.Request) {
	if r.Method != "POST" {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}
	var req struct {
		Operations int `json:"operations"`
	}
	json.NewDecoder(r.Body).Decode(&req)
	if req.Operations <= 0 {
		req.Operations = 1000
	}
	if req.Operations > 100000 {
		req.Operations = 100000
	}

	result := s.engine.RunBenchmark(req.Operations)
	writeJSON(w, result)
}

// parseCommand parses a command string into parts, handling quoted strings.
func parseCommand(input string) []string {
	var parts []string
	var current strings.Builder
	inQuote := false
	quoteChar := byte(0)

	for i := 0; i < len(input); i++ {
		c := input[i]
		if inQuote {
			if c == quoteChar {
				inQuote = false
			} else {
				current.WriteByte(c)
			}
		} else if c == '"' || c == '\'' {
			inQuote = true
			quoteChar = c
		} else if c == ' ' {
			if current.Len() > 0 {
				parts = append(parts, current.String())
				current.Reset()
			}
		} else {
			current.WriteByte(c)
		}
	}
	if current.Len() > 0 {
		parts = append(parts, current.String())
	}
	return parts
}

// writeJSON writes a JSON response.
func writeJSON(w http.ResponseWriter, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

func writeJSONWithStatus(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

// formatDuration formats a duration as human-readable string.
func formatDuration(d time.Duration) string {
	days := int(d.Hours() / 24)
	hours := int(d.Hours()) % 24
	mins := int(d.Minutes()) % 60
	secs := int(d.Seconds()) % 60

	if days > 0 {
		return fmt.Sprintf("%dd %dh %dm %ds", days, hours, mins, secs)
	}
	if hours > 0 {
		return fmt.Sprintf("%dh %dm %ds", hours, mins, secs)
	}
	if mins > 0 {
		return fmt.Sprintf("%dm %ds", mins, secs)
	}
	return fmt.Sprintf("%ds", secs)
}
