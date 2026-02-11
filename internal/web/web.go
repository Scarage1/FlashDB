// Package web provides the HTTP web interface for FlashDB.
package web

import (
	"context"
	"embed"
	"encoding/json"
	"fmt"
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
)

//go:embed static/*
var staticFiles embed.FS

// Server represents the web server for FlashDB admin interface.
type Server struct {
	addr      string
	engine    *engine.Engine
	server    *http.Server
	startTime time.Time
	mu        sync.RWMutex
}

const apiVersionPath = "/api/v1"

// New creates a new web server.
func New(addr string, engine *engine.Engine) *Server {
	return &Server{
		addr:      addr,
		engine:    engine,
		startTime: time.Now(),
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
	mux.Handle("/", http.FileServer(http.FS(staticFS)))

	s.server = &http.Server{
		Addr:    s.addr,
		Handler: corsMiddleware(mux),
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

	return mux
}

// corsMiddleware adds CORS headers.
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type")

		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
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
		return s.engine.ZAdd(key, members...), nil

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

	case "INFO":
		stats := s.engine.GetStats()
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		info := fmt.Sprintf("# Server\nflashdb_version:1.0.0\nuptime_in_seconds:%d\n\n# Keyspace\nkeys:%d\ntotal_reads:%d\ntotal_writes:%d\n\n# Memory\nused_memory:%d\n",
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
		Version:       "1.0.0",
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
			Type: "string",
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
		val, exists := s.engine.Get(key)
		if !exists {
			http.Error(w, "Key not found", http.StatusNotFound)
			return
		}
		ttl := s.engine.TTL(key)
		writeJSON(w, KeyInfo{
			Key:   key,
			Type:  "string",
			TTL:   ttl,
			Value: string(val),
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
