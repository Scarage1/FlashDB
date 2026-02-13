// Package server implements the TCP server for FlashDB using RESP protocol.
package server

import (
	"bytes"
	"context"
	"crypto/subtle"
	"crypto/tls"
	"errors"
	"fmt"
	"io"
	"log"
	"log/slog"
	"math"
	"math/rand"
	"net"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/flashdb/flashdb/internal/protocol"
	"github.com/flashdb/flashdb/internal/store"
	"github.com/flashdb/flashdb/internal/version"
)

// Version is the FlashDB version string.
var Version = version.Version

// ACLUser represents a user in the ACL system.
type ACLUser struct {
	Username    string
	Password    string // plaintext for Redis compat (compared constant-time)
	Enabled     bool
	AllCommands bool     // true = unrestricted
	AllowedCmds []string // if AllCommands is false, whitelist
	ReadOnly    bool     // true = only read commands allowed
}

// slowLogEntry records a command that exceeded the latency threshold.
type slowLogEntry struct {
	ID        int64
	Timestamp time.Time
	Duration  time.Duration
	Client    string
	Cmd       string
	Args      []string
}

// Config holds server configuration.
type Config struct {
	Password   string
	MaxClients int
	Timeout    time.Duration
	LogLevel   string

	// TLS
	TLSCertFile string
	TLSKeyFile  string

	// ACL — when Users is non-empty, per-user auth is used instead of Password.
	Users []ACLUser

	// Rate limiting — max commands per second per client (0 = unlimited).
	RateLimit int

	// Slow query log — commands slower than this are recorded (0 = disabled).
	SlowLogThreshold time.Duration
	SlowLogMaxLen    int

	// Web API token (shared secret for HTTP endpoints, empty = no auth).
	APIToken string
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Password:         "",
		MaxClients:       10000,
		Timeout:          0,
		LogLevel:         "info",
		SlowLogMaxLen:    128,
		SlowLogThreshold: 0,
		RateLimit:        0,
	}
}

// clientConn represents a client connection with state.
type clientConn struct {
	id            int64
	conn          net.Conn
	writeMu       sync.Mutex
	addr          string
	authenticated bool
	createdAt     time.Time
	lastCommand   time.Time
	cmdCount      int64
	// Transaction state
	inMulti    bool
	multiQueue []queuedCommand
	// Pub/Sub state
	subscriptions  map[string]bool
	psubscriptions map[string]bool
	// ACL state
	aclUser *ACLUser // nil when legacy single-password mode
	// Rate limiting state
	rateBucket  int64 // remaining tokens this second
	rateResetAt time.Time
}

// queuedCommand represents a command queued during MULTI
type queuedCommand struct {
	cmd  string
	args []protocol.Value
}

// PubSub manages publish/subscribe functionality
type PubSub struct {
	mu       sync.RWMutex
	channels map[string]map[*clientConn]bool
	patterns map[string]map[*clientConn]bool
}

// NewPubSub creates a new PubSub manager
func NewPubSub() *PubSub {
	return &PubSub{
		channels: make(map[string]map[*clientConn]bool),
		patterns: make(map[string]map[*clientConn]bool),
	}
}

// Server represents the FlashDB TCP server.
type Server struct {
	addr       string
	engine     *engine.Engine
	config     Config
	listener   net.Listener
	wg         sync.WaitGroup
	mu         sync.RWMutex
	closed     bool
	connCount  int64
	nextConnID int64
	clients    map[int64]*clientConn
	startTime  time.Time
	totalCmds  int64
	totalConns int64
	pubsub     *PubSub
	// Slow query log
	slowLog   []slowLogEntry
	slowLogMu sync.Mutex
	slowLogID int64
	// Structured logger
	logger *slog.Logger
}

// New creates a new Server with the specified address and engine.
func New(addr string, e *engine.Engine) *Server {
	return NewWithConfig(addr, e, DefaultConfig())
}

// NewWithConfig creates a new Server with the specified configuration.
func NewWithConfig(addr string, e *engine.Engine, cfg Config) *Server {
	// Build structured logger based on configured level.
	var level slog.Level
	switch strings.ToLower(cfg.LogLevel) {
	case "debug":
		level = slog.LevelDebug
	case "warn", "warning":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}
	logger := slog.New(slog.NewJSONHandler(log.Writer(), &slog.HandlerOptions{Level: level}))

	return &Server{
		addr:      addr,
		engine:    e,
		config:    cfg,
		clients:   make(map[int64]*clientConn),
		startTime: time.Now(),
		pubsub:    NewPubSub(),
		logger:    logger,
	}
}

// Start starts the server and listens for connections.
// It blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	addr, err := net.ResolveTCPAddr("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("server: failed to resolve address: %w", err)
	}
	tcpListener, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return fmt.Errorf("server: failed to listen: %w", err)
	}

	// Optionally wrap with TLS.
	var listener net.Listener = tcpListener
	if s.config.TLSCertFile != "" && s.config.TLSKeyFile != "" {
		cert, err := tls.LoadX509KeyPair(s.config.TLSCertFile, s.config.TLSKeyFile)
		if err != nil {
			tcpListener.Close()
			return fmt.Errorf("server: failed to load TLS certificate: %w", err)
		}
		tlsCfg := &tls.Config{
			Certificates: []tls.Certificate{cert},
			MinVersion:   tls.VersionTLS12,
		}
		listener = tls.NewListener(tcpListener, tlsCfg)
		s.logger.Info("TLS enabled", "cert", s.config.TLSCertFile)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	s.logger.Info("FlashDB server listening", "addr", s.addr)
	if s.config.Password != "" || len(s.config.Users) > 0 {
		s.logger.Info("Authentication enabled")
	}

	// Handle context cancellation
	go func() {
		<-ctx.Done()
		s.Close()
	}()

	for {
		conn, err := listener.Accept()
		if err != nil {
			s.mu.Lock()
			closed := s.closed
			s.mu.Unlock()

			if closed {
				return nil
			}
			s.logger.Error("failed to accept connection", "error", err)
			continue
		}

		// TCP socket tuning — works for plain TCP and TLS-wrapped conns.
		if tc, ok := conn.(*net.TCPConn); ok {
			tc.SetNoDelay(true)
			tc.SetKeepAlive(true)
			tc.SetKeepAlivePeriod(5 * time.Minute)
		}

		// Check max clients
		s.mu.RLock()
		currentClients := len(s.clients)
		s.mu.RUnlock()

		if s.config.MaxClients > 0 && currentClients >= s.config.MaxClients {
			conn.Close()
			s.logger.Warn("max clients reached, rejecting connection")
			continue
		}

		// Create client connection
		noAuth := s.config.Password == "" && len(s.config.Users) == 0
		s.mu.Lock()
		s.nextConnID++
		connID := s.nextConnID
		client := &clientConn{
			id:             connID,
			conn:           conn,
			addr:           conn.RemoteAddr().String(),
			authenticated:  noAuth,
			createdAt:      time.Now(),
			lastCommand:    time.Now(),
			subscriptions:  make(map[string]bool),
			psubscriptions: make(map[string]bool),
		}
		s.clients[connID] = client
		s.connCount++
		s.totalConns++
		s.mu.Unlock()

		s.wg.Add(1)
		go func(c *clientConn) {
			defer s.wg.Done()
			defer func() {
				// Clean up pub/sub subscriptions
				s.pubsub.UnsubscribeAll(c)
				s.mu.Lock()
				delete(s.clients, c.id)
				s.connCount--
				s.mu.Unlock()
			}()
			s.handleConnection(ctx, c)
		}(client)
	}
}

// Close gracefully shuts down the server.
func (s *Server) Close() error {
	s.mu.Lock()
	if s.closed {
		s.mu.Unlock()
		return nil
	}
	s.closed = true
	listener := s.listener
	s.mu.Unlock()

	var err error
	if listener != nil {
		err = listener.Close()
	}

	// Wait for all connections to finish
	s.wg.Wait()

	return err
}

// handleConnection handles a single client connection.
func (s *Server) handleConnection(ctx context.Context, client *clientConn) {
	defer client.conn.Close()

	reader := protocol.NewReader(client.conn)
	writer := protocol.NewWriter(client.conn)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		// Set read timeout if configured
		if s.config.Timeout > 0 {
			client.conn.SetReadDeadline(time.Now().Add(s.config.Timeout))
		}

		val, err := reader.ReadValue()
		if err != nil {
			if err != io.EOF && !errors.Is(err, net.ErrClosed) {
				if !strings.Contains(err.Error(), "timeout") {
					log.Printf("server: failed to read: %v", err)
				}
			}
			return
		}

		if val.Type != protocol.TypeArray || len(val.Array) == 0 {
			writer.WriteError("invalid command format")
			continue
		}

		// If more data is already buffered, enter pipeline mode:
		// disable per-command flush, drain all buffered commands,
		// then flush the entire batch in one syscall.
		pipelined := reader.Buffered() > 0
		if pipelined {
			writer.SetAutoFlush(false)
		}

		s.dispatchCommand(writer, client, val)

		// Drain any remaining pipelined commands
		for pipelined && reader.Buffered() > 0 {
			val, err = reader.ReadValue()
			if err != nil {
				break
			}
			if val.Type != protocol.TypeArray || len(val.Array) == 0 {
				writer.WriteError("invalid command format")
				continue
			}
			s.dispatchCommand(writer, client, val)
		}

		if pipelined {
			writer.SetAutoFlush(true)
			writer.Flush()
		}
	}
}

// dispatchCommand parses a RESP array value into command + args and executes it.
// It enforces rate limiting, ACL, slow-log recording, and audit logging.
func (s *Server) dispatchCommand(w *protocol.Writer, client *clientConn, val protocol.Value) {
	cmd := strings.ToUpper(val.Array[0].Str)
	args := val.Array[1:]

	// Update client stats
	now := time.Now()
	client.lastCommand = now
	atomic.AddInt64(&client.cmdCount, 1)
	atomic.AddInt64(&s.totalCmds, 1)

	// --- Rate limiting (token-bucket, 1-second window) ---
	if s.config.RateLimit > 0 {
		if now.After(client.rateResetAt) {
			client.rateBucket = int64(s.config.RateLimit)
			client.rateResetAt = now.Add(time.Second)
		}
		if client.rateBucket <= 0 {
			w.WriteError("ERR rate limit exceeded, try again later")
			return
		}
		client.rateBucket--
	}

	// --- Authentication check ---
	if !client.authenticated && cmd != "AUTH" && cmd != "PING" && cmd != "QUIT" {
		w.WriteError("NOAUTH Authentication required")
		return
	}

	// --- ACL permission check ---
	if client.aclUser != nil && !client.aclUser.AllCommands {
		if !s.aclAllowed(client.aclUser, cmd) {
			w.WriteError(fmt.Sprintf("NOPERM this user has no permissions to run the '%s' command", strings.ToLower(cmd)))
			s.logger.Warn("ACL denied", "user", client.aclUser.Username, "cmd", cmd, "client", client.addr)
			return
		}
	}

	// --- Execute with slow-log timing ---
	start := time.Now()
	s.executeCommand(w, client, cmd, args)
	elapsed := time.Since(start)

	// Record slow queries.
	if s.config.SlowLogThreshold > 0 && elapsed >= s.config.SlowLogThreshold {
		argStrs := make([]string, len(args))
		for i, a := range args {
			argStrs[i] = a.Str
		}
		s.addSlowLog(elapsed, client.addr, cmd, argStrs)
	}

	// --- Audit logging for security-sensitive commands ---
	switch cmd {
	case "AUTH", "FLUSHDB", "FLUSHALL", "CONFIG", "ACL", "DEBUG", "SAVE", "BGSAVE", "SHUTDOWN":
		user := "default"
		if client.aclUser != nil {
			user = client.aclUser.Username
		}
		s.logger.Info("audit",
			"cmd", cmd,
			"user", user,
			"client", client.addr,
			"latency_us", elapsed.Microseconds(),
		)
	}
}

// executeCommand executes a command and writes the response.
func (s *Server) executeCommand(w *protocol.Writer, client *clientConn, cmd string, args []protocol.Value) {
	// Handle MULTI transaction queueing
	if client.inMulti && cmd != "EXEC" && cmd != "DISCARD" && cmd != "MULTI" {
		client.multiQueue = append(client.multiQueue, queuedCommand{cmd: cmd, args: args})
		w.WriteSimpleString("QUEUED")
		return
	}

	switch cmd {
	// Connection
	case "PING":
		s.cmdPing(w, args)
	case "ECHO":
		s.cmdEcho(w, args)
	case "QUIT":
		s.cmdQuit(w)
	case "AUTH":
		s.cmdAuth(w, client, args)
	case "SELECT":
		s.cmdSelect(w, args)
	case "CLIENT":
		s.cmdClient(w, client, args)

	// Transaction commands
	case "MULTI":
		s.cmdMulti(w, client)
	case "EXEC":
		s.cmdExec(w, client)
	case "DISCARD":
		s.cmdDiscard(w, client)

	// String commands
	case "SET":
		s.cmdSet(w, args)
	case "GET":
		s.cmdGet(w, args)
	case "GETSET":
		s.cmdGetSet(w, args)
	case "GETEX":
		s.cmdGetEx(w, args)
	case "GETDEL":
		s.cmdGetDel(w, args)
	case "GETRANGE":
		s.cmdGetRange(w, args)
	case "SETRANGE":
		s.cmdSetRange(w, args)
	case "SETNX":
		s.cmdSetNX(w, args)
	case "SETEX":
		s.cmdSetEX(w, args)
	case "PSETEX":
		s.cmdPSetEX(w, args)
	case "MSET":
		s.cmdMSet(w, args)
	case "MGET":
		s.cmdMGet(w, args)
	case "MSETNX":
		s.cmdMSetNX(w, args)
	case "APPEND":
		s.cmdAppend(w, args)
	case "STRLEN":
		s.cmdStrLen(w, args)
	case "INCR":
		s.cmdIncr(w, args)
	case "INCRBY":
		s.cmdIncrBy(w, args)
	case "INCRBYFLOAT":
		s.cmdIncrByFloat(w, args)
	case "DECR":
		s.cmdDecr(w, args)
	case "DECRBY":
		s.cmdDecrBy(w, args)

	// Key commands
	case "DEL":
		s.cmdDel(w, args)
	case "UNLINK":
		s.cmdDel(w, args) // Same as DEL in our implementation
	case "EXISTS":
		s.cmdExists(w, args)
	case "KEYS":
		s.cmdKeys(w, args)
	case "SCAN":
		s.cmdScan(w, args)
	case "EXPIRE":
		s.cmdExpire(w, args)
	case "PEXPIRE":
		s.cmdPExpire(w, args)
	case "EXPIREAT":
		s.cmdExpireAt(w, args)
	case "PEXPIREAT":
		s.cmdPExpireAt(w, args)
	case "TTL":
		s.cmdTTL(w, args)
	case "PTTL":
		s.cmdPTTL(w, args)
	case "PERSIST":
		s.cmdPersist(w, args)
	case "TYPE":
		s.cmdType(w, args)
	case "RENAME":
		s.cmdRename(w, args)
	case "RENAMENX":
		s.cmdRenameNX(w, args)
	case "RANDOMKEY":
		s.cmdRandomKey(w)
	case "TOUCH":
		s.cmdTouch(w, args)
	case "OBJECT":
		s.cmdObject(w, args)
	case "DUMP":
		s.cmdDump(w, args)
	case "COPY":
		s.cmdCopy(w, args)

	// Pub/Sub commands
	case "PUBLISH":
		s.cmdPublish(w, args)
	case "SUBSCRIBE":
		s.cmdSubscribe(w, client, args)
	case "UNSUBSCRIBE":
		s.cmdUnsubscribe(w, client, args)
	case "PSUBSCRIBE":
		s.cmdPSubscribe(w, client, args)
	case "PUNSUBSCRIBE":
		s.cmdPUnsubscribe(w, client, args)
	case "PUBSUB":
		s.cmdPubSub(w, args)

	// Server commands
	case "DBSIZE":
		s.cmdDBSize(w, args)
	case "FLUSHDB", "FLUSHALL":
		s.cmdFlushDB(w, args)
	case "INFO":
		s.cmdInfo(w, args)
	case "TIME":
		s.cmdTime(w)
	case "COMMAND":
		s.cmdCommand(w)
	case "CONFIG":
		s.cmdConfig(w, args)
	case "DEBUG":
		s.cmdDebug(w, args)
	case "MEMORY":
		s.cmdMemory(w, args)
	case "LASTSAVE":
		s.cmdLastSave(w)
	case "BGSAVE", "SAVE":
		s.cmdSave(w)
	case "SLOWLOG":
		s.cmdSlowLog(w, args)
	case "ACL":
		s.cmdACL(w, client, args)

	// Sorted Set commands
	case "ZADD":
		s.cmdZAdd(w, args)
	case "ZSCORE":
		s.cmdZScore(w, args)
	case "ZREM":
		s.cmdZRem(w, args)
	case "ZCARD":
		s.cmdZCard(w, args)
	case "ZRANK":
		s.cmdZRank(w, args)
	case "ZREVRANK":
		s.cmdZRevRank(w, args)
	case "ZRANGE":
		s.cmdZRange(w, args)
	case "ZREVRANGE":
		s.cmdZRevRange(w, args)
	case "ZRANGEBYSCORE":
		s.cmdZRangeByScore(w, args)
	case "ZREVRANGEBYSCORE":
		s.cmdZRevRangeByScore(w, args)
	case "ZCOUNT":
		s.cmdZCount(w, args)
	case "ZINCRBY":
		s.cmdZIncrBy(w, args)
	case "ZREMRANGEBYRANK":
		s.cmdZRemRangeByRank(w, args)
	case "ZREMRANGEBYSCORE":
		s.cmdZRemRangeByScore(w, args)
	case "ZPOPMIN":
		s.cmdZPopMin(w, args)
	case "ZPOPMAX":
		s.cmdZPopMax(w, args)

	// Hash commands
	case "HSET":
		s.cmdHSet(w, args)
	case "HGET":
		s.cmdHGet(w, args)
	case "HMSET":
		s.cmdHMSet(w, args)
	case "HMGET":
		s.cmdHMGet(w, args)
	case "HDEL":
		s.cmdHDel(w, args)
	case "HEXISTS":
		s.cmdHExists(w, args)
	case "HLEN":
		s.cmdHLen(w, args)
	case "HGETALL":
		s.cmdHGetAll(w, args)
	case "HKEYS":
		s.cmdHKeys(w, args)
	case "HVALS":
		s.cmdHVals(w, args)
	case "HINCRBY":
		s.cmdHIncrBy(w, args)
	case "HINCRBYFLOAT":
		s.cmdHIncrByFloat(w, args)
	case "HSETNX":
		s.cmdHSetNX(w, args)

	// List commands
	case "LPUSH":
		s.cmdLPush(w, args)
	case "RPUSH":
		s.cmdRPush(w, args)
	case "LPOP":
		s.cmdLPop(w, args)
	case "RPOP":
		s.cmdRPop(w, args)
	case "LLEN":
		s.cmdLLen(w, args)
	case "LINDEX":
		s.cmdLIndex(w, args)
	case "LSET":
		s.cmdLSet(w, args)
	case "LRANGE":
		s.cmdLRange(w, args)
	case "LINSERT":
		s.cmdLInsert(w, args)
	case "LREM":
		s.cmdLRem(w, args)
	case "LTRIM":
		s.cmdLTrim(w, args)

	// Set commands
	case "SADD":
		s.cmdSAdd(w, args)
	case "SREM":
		s.cmdSRem(w, args)
	case "SISMEMBER":
		s.cmdSIsMember(w, args)
	case "SCARD":
		s.cmdSCard(w, args)
	case "SMEMBERS":
		s.cmdSMembers(w, args)
	case "SRANDMEMBER":
		s.cmdSRandMember(w, args)
	case "SPOP":
		s.cmdSPop(w, args)
	case "SINTER":
		s.cmdSInter(w, args)
	case "SUNION":
		s.cmdSUnion(w, args)
	case "SDIFF":
		s.cmdSDiff(w, args)

	// Time-Series commands
	case "TS.ADD":
		s.cmdTSAdd(w, args)
	case "TS.GET":
		s.cmdTSGet(w, args)
	case "TS.RANGE":
		s.cmdTSRange(w, args)
	case "TS.INFO":
		s.cmdTSInfo(w, args)
	case "TS.DEL":
		s.cmdTSDel(w, args)
	case "TS.KEYS":
		s.cmdTSKeys(w)

	// Hot key detection
	case "HOTKEYS":
		s.cmdHotKeys(w, args)

	// Snapshot commands
	case "SNAPSHOT":
		s.cmdSnapshot(w, args)

	// CDC commands
	case "CDC":
		s.cmdCDC(w, args)

	// Benchmark
	case "BENCHMARK":
		s.cmdBenchmark(w, args)

	default:
		w.WriteError(fmt.Sprintf("unknown command '%s'", cmd))
	}
}

// Connection commands

func (s *Server) cmdPing(w *protocol.Writer, args []protocol.Value) {
	if len(args) > 1 {
		w.WriteError("wrong number of arguments for 'PING' command")
		return
	}
	if len(args) == 1 {
		w.WriteBulkString([]byte(args[0].Str))
		return
	}
	w.WriteSimpleString("PONG")
}

func (s *Server) cmdEcho(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'ECHO' command")
		return
	}
	w.WriteBulkString([]byte(args[0].Str))
}

func (s *Server) cmdQuit(w *protocol.Writer) {
	w.WriteSimpleString("OK")
}

// String commands

func (s *Server) cmdSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'SET' command")
		return
	}

	key := args[0].Str
	value := []byte(args[1].Str)

	// Parse optional arguments: EX seconds | PX milliseconds | NX | XX
	var ttl time.Duration
	nx, xx := false, false
	expireOptionSet := false

	for i := 2; i < len(args); i++ {
		opt := strings.ToUpper(args[i].Str)
		switch opt {
		case "EX":
			if i+1 >= len(args) {
				w.WriteError("syntax error")
				return
			}
			if expireOptionSet {
				w.WriteError("syntax error")
				return
			}
			seconds, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil || seconds <= 0 {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Duration(seconds) * time.Second
			expireOptionSet = true
			i++
		case "PX":
			if i+1 >= len(args) {
				w.WriteError("syntax error")
				return
			}
			if expireOptionSet {
				w.WriteError("syntax error")
				return
			}
			millis, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil || millis <= 0 {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Duration(millis) * time.Millisecond
			expireOptionSet = true
			i++
		case "NX":
			if nx {
				w.WriteError("syntax error")
				return
			}
			nx = true
		case "XX":
			if xx {
				w.WriteError("syntax error")
				return
			}
			xx = true
		default:
			w.WriteError("syntax error")
			return
		}
	}

	if nx && xx {
		w.WriteError("XX and NX options are mutually exclusive")
		return
	}

	if nx {
		exists := s.engine.Exists(key)
		if exists {
			w.WriteNull()
			return
		}
	}

	if xx {
		exists := s.engine.Exists(key)
		if !exists {
			w.WriteNull()
			return
		}
	}

	var err error
	if ttl > 0 {
		err = s.engine.SetWithTTL(key, value, ttl)
	} else {
		err = s.engine.Set(key, value)
	}

	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: SET error: %v", err)
		return
	}

	w.WriteSimpleString("OK")
}

func (s *Server) cmdGet(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'GET' command")
		return
	}

	key := args[0].Str
	value, ok := s.engine.Get(key)

	if !ok {
		w.WriteNull()
		return
	}

	w.WriteBulkString(value)
}

func (s *Server) cmdSetNX(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'SETNX' command")
		return
	}

	key := args[0].Str
	value := []byte(args[1].Str)

	set, err := s.engine.SetNX(key, value)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if set {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdSetEX(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'SETEX' command")
		return
	}

	key := args[0].Str
	seconds, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil || seconds <= 0 {
		w.WriteError("invalid expire time")
		return
	}
	value := []byte(args[2].Str)

	if err := s.engine.SetWithTTL(key, value, time.Duration(seconds)*time.Second); err != nil {
		w.WriteError("internal error")
		return
	}

	w.WriteSimpleString("OK")
}

func (s *Server) cmdPSetEX(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'PSETEX' command")
		return
	}

	key := args[0].Str
	millis, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil || millis <= 0 {
		w.WriteError("invalid expire time")
		return
	}
	value := []byte(args[2].Str)

	if err := s.engine.SetWithTTL(key, value, time.Duration(millis)*time.Millisecond); err != nil {
		w.WriteError("internal error")
		return
	}

	w.WriteSimpleString("OK")
}

func (s *Server) cmdMSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 || len(args)%2 != 0 {
		w.WriteError("wrong number of arguments for 'MSET' command")
		return
	}

	pairs := make(map[string][]byte, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		pairs[args[i].Str] = []byte(args[i+1].Str)
	}

	if err := s.engine.MSet(pairs); err != nil {
		w.WriteError("internal error")
		return
	}

	w.WriteSimpleString("OK")
}

func (s *Server) cmdMGet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'MGET' command")
		return
	}

	results := make([][]byte, len(args))
	nulls := make([]bool, len(args))

	for i, arg := range args {
		value, ok := s.engine.Get(arg.Str)
		if ok {
			results[i] = value
		} else {
			nulls[i] = true
		}
	}

	w.WriteArrayWithNulls(results, nulls)
}

func (s *Server) cmdAppend(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'APPEND' command")
		return
	}

	key := args[0].Str
	value := []byte(args[1].Str)

	length, err := s.engine.Append(key, value)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	w.WriteInteger(int64(length))
}

func (s *Server) cmdStrLen(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'STRLEN' command")
		return
	}

	length := s.engine.StrLen(args[0].Str)
	w.WriteInteger(int64(length))
}

func (s *Server) cmdIncr(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'INCR' command")
		return
	}

	val, err := s.engine.IncrBy(args[0].Str, 1)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	w.WriteInteger(val)
}

func (s *Server) cmdIncrBy(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'INCRBY' command")
		return
	}

	delta, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("value is not an integer")
		return
	}

	val, err := s.engine.IncrBy(args[0].Str, delta)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	w.WriteInteger(val)
}

func (s *Server) cmdDecr(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'DECR' command")
		return
	}

	val, err := s.engine.IncrBy(args[0].Str, -1)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	w.WriteInteger(val)
}

func (s *Server) cmdDecrBy(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'DECRBY' command")
		return
	}

	delta, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("value is not an integer")
		return
	}

	val, err := s.engine.IncrBy(args[0].Str, -delta)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	w.WriteInteger(val)
}

// Key commands

func (s *Server) cmdDel(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'DEL' command")
		return
	}

	var count int64
	for _, arg := range args {
		deleted, err := s.engine.Delete(arg.Str)
		if err != nil {
			w.WriteError("internal error")
			log.Printf("server: DEL error: %v", err)
			return
		}
		if deleted {
			count++
		}
	}

	w.WriteInteger(count)
}

func (s *Server) cmdExists(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'EXISTS' command")
		return
	}

	var count int64
	for _, arg := range args {
		if s.engine.Exists(arg.Str) {
			count++
		}
	}

	w.WriteInteger(count)
}

func (s *Server) cmdKeys(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'KEYS' command")
		return
	}

	pattern := args[0].Str
	keys := s.engine.Keys()

	if pattern == "*" {
		w.WriteStringArray(keys)
		return
	}

	// Use filepath.Match for Redis-compatible glob matching
	var matched []string
	for _, key := range keys {
		if ok, _ := filepath.Match(pattern, key); ok {
			matched = append(matched, key)
		}
	}
	if matched == nil {
		matched = []string{}
	}

	w.WriteStringArray(matched)
}

func (s *Server) cmdExpire(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'EXPIRE' command")
		return
	}

	key := args[0].Str
	seconds, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("value is not an integer")
		return
	}

	set, err := s.engine.Expire(key, time.Duration(seconds)*time.Second)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if set {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdPExpire(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'PEXPIRE' command")
		return
	}

	key := args[0].Str
	millis, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("value is not an integer")
		return
	}

	set, err := s.engine.Expire(key, time.Duration(millis)*time.Millisecond)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if set {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdTTL(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'TTL' command")
		return
	}

	ttl := s.engine.TTL(args[0].Str)
	w.WriteInteger(ttl)
}

func (s *Server) cmdPTTL(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'PTTL' command")
		return
	}

	ttl := s.engine.PTTL(args[0].Str)
	w.WriteInteger(ttl)
}

func (s *Server) cmdPersist(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'PERSIST' command")
		return
	}

	persisted, err := s.engine.Persist(args[0].Str)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if persisted {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdType(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'TYPE' command")
		return
	}

	w.WriteSimpleString(s.engine.KeyType(args[0].Str))
}

// Server commands

func (s *Server) cmdDBSize(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 0 {
		w.WriteError("wrong number of arguments for 'DBSIZE' command")
		return
	}

	w.WriteInteger(int64(s.engine.Size()))
}

func (s *Server) cmdFlushDB(w *protocol.Writer, args []protocol.Value) {
	if err := s.engine.Clear(); err != nil {
		w.WriteError("internal error")
		log.Printf("server: FLUSHDB error: %v", err)
		return
	}

	w.WriteSimpleString("OK")
}

func (s *Server) cmdInfo(w *protocol.Writer, args []protocol.Value) {
	stats := s.engine.GetStats()
	uptime := time.Since(stats.StartTime).Seconds()

	s.mu.Lock()
	connCount := s.connCount
	s.mu.Unlock()

	info := fmt.Sprintf(`# Server
flashdb_version:%s
uptime_in_seconds:%.0f
connected_clients:%d

# Stats
total_commands_processed:%d
total_reads:%d
total_writes:%d

# Keyspace
db0:keys=%d
`, Version, uptime, connCount, stats.TotalCommands, stats.TotalReads, stats.TotalWrites, stats.KeysCount)

	w.WriteBulkString([]byte(info))
}

func (s *Server) cmdTime(w *protocol.Writer) {
	now := time.Now()
	secs := fmt.Sprintf("%d", now.Unix())
	micros := fmt.Sprintf("%d", now.Nanosecond()/1000)
	w.WriteStringArray([]string{secs, micros})
}

func (s *Server) cmdCommand(w *protocol.Writer) {
	// Return empty array for COMMAND - just for compatibility
	w.WriteStringArray([]string{})
}

// AUTH command — supports both legacy single-password and ACL user/password.
// AUTH <password>         (legacy mode)
// AUTH <username> <password>  (ACL mode)
func (s *Server) cmdAuth(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) < 1 || len(args) > 2 {
		w.WriteError("wrong number of arguments for 'AUTH' command")
		return
	}

	// ACL mode: per-user authentication
	if len(s.config.Users) > 0 {
		var username, password string
		if len(args) == 2 {
			username = args[0].Str
			password = args[1].Str
		} else {
			username = "default"
			password = args[0].Str
		}
		for i := range s.config.Users {
			u := &s.config.Users[i]
			if u.Username == username && u.Enabled {
				if subtle.ConstantTimeCompare([]byte(u.Password), []byte(password)) == 1 {
					client.authenticated = true
					client.aclUser = u
					w.WriteSimpleString("OK")
					return
				}
			}
		}
		w.WriteError("WRONGPASS invalid username-password pair or user is disabled")
		return
	}

	// Legacy single-password mode
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'AUTH' command")
		return
	}
	if s.config.Password == "" {
		w.WriteError("Client sent AUTH, but no password is set")
		return
	}
	if subtle.ConstantTimeCompare([]byte(args[0].Str), []byte(s.config.Password)) == 1 {
		client.authenticated = true
		w.WriteSimpleString("OK")
	} else {
		w.WriteError("WRONGPASS invalid username-password pair")
	}
}

// ---------- ACL helpers ----------

// readOnlyCmds are commands that only read data (no mutation).
var readOnlyCmds = map[string]bool{
	"GET": true, "MGET": true, "STRLEN": true, "GETRANGE": true,
	"EXISTS": true, "KEYS": true, "SCAN": true, "TTL": true, "PTTL": true,
	"TYPE": true, "RANDOMKEY": true, "DBSIZE": true, "INFO": true,
	"PING": true, "ECHO": true, "TIME": true, "COMMAND": true,
	"CLIENT": true, "SELECT": true, "AUTH": true, "QUIT": true,
	"OBJECT": true, "DUMP": true, "TOUCH": true, "LLEN": true,
	"LINDEX": true, "LRANGE": true, "SISMEMBER": true, "SCARD": true,
	"SMEMBERS": true, "SRANDMEMBER": true, "SINTER": true, "SUNION": true,
	"SDIFF": true, "HGET": true, "HMGET": true, "HEXISTS": true,
	"HLEN": true, "HGETALL": true, "HKEYS": true, "HVALS": true,
	"ZSCORE": true, "ZCARD": true, "ZRANK": true, "ZREVRANK": true,
	"ZRANGE": true, "ZREVRANGE": true, "ZRANGEBYSCORE": true,
	"ZREVRANGEBYSCORE": true, "ZCOUNT": true, "SUBSCRIBE": true,
	"PSUBSCRIBE": true, "PUBSUB": true, "SLOWLOG": true,
}

func (s *Server) aclAllowed(u *ACLUser, cmd string) bool {
	if u.AllCommands {
		return true
	}
	if u.ReadOnly {
		return readOnlyCmds[cmd]
	}
	for _, allowed := range u.AllowedCmds {
		if strings.EqualFold(allowed, cmd) {
			return true
		}
	}
	return false
}

// cmdACL implements the ACL WHOAMI / ACL LIST commands.
func (s *Server) cmdACL(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'ACL' command")
		return
	}
	sub := strings.ToUpper(args[0].Str)
	switch sub {
	case "WHOAMI":
		if client.aclUser != nil {
			w.WriteBulkString([]byte(client.aclUser.Username))
		} else {
			w.WriteBulkString([]byte("default"))
		}
	case "LIST":
		items := make([]string, 0, len(s.config.Users))
		for _, u := range s.config.Users {
			status := "on"
			if !u.Enabled {
				status = "off"
			}
			perms := "~* +@all"
			if u.ReadOnly {
				perms = "~* +@read"
			} else if !u.AllCommands {
				perms = "~* +" + strings.Join(u.AllowedCmds, " +")
			}
			items = append(items, fmt.Sprintf("user %s %s %s", u.Username, status, perms))
		}
		w.WriteStringArray(items)
	default:
		w.WriteError(fmt.Sprintf("unknown subcommand '%s' for ACL", sub))
	}
}

// ---------- Slow-log ----------

func (s *Server) addSlowLog(dur time.Duration, client, cmd string, args []string) {
	s.slowLogMu.Lock()
	defer s.slowLogMu.Unlock()
	s.slowLogID++
	entry := slowLogEntry{
		ID:        s.slowLogID,
		Timestamp: time.Now(),
		Duration:  dur,
		Client:    client,
		Cmd:       cmd,
		Args:      args,
	}
	s.slowLog = append(s.slowLog, entry)
	if len(s.slowLog) > s.config.SlowLogMaxLen {
		s.slowLog = s.slowLog[len(s.slowLog)-s.config.SlowLogMaxLen:]
	}
	s.logger.Warn("slow query",
		"id", entry.ID,
		"cmd", cmd,
		"duration_us", dur.Microseconds(),
		"client", client,
	)
}

// cmdSlowLog implements SLOWLOG GET [count] / SLOWLOG LEN / SLOWLOG RESET.
func (s *Server) cmdSlowLog(w *protocol.Writer, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'SLOWLOG' command")
		return
	}
	sub := strings.ToUpper(args[0].Str)
	switch sub {
	case "GET":
		count := 10
		if len(args) > 1 {
			if n, err := strconv.Atoi(args[1].Str); err == nil && n > 0 {
				count = n
			}
		}
		s.slowLogMu.Lock()
		entries := s.slowLog
		if len(entries) > count {
			entries = entries[len(entries)-count:]
		}
		// Build response: array of arrays
		result := make([]string, 0, len(entries)*4)
		for i := len(entries) - 1; i >= 0; i-- {
			e := entries[i]
			result = append(result,
				fmt.Sprintf("%d) id=%d, time=%d, duration=%dus, client=%s, cmd=%s %s",
					len(entries)-i, e.ID, e.Timestamp.Unix(), e.Duration.Microseconds(),
					e.Client, e.Cmd, strings.Join(e.Args, " ")))
		}
		s.slowLogMu.Unlock()
		w.WriteStringArray(result)
	case "LEN":
		s.slowLogMu.Lock()
		n := len(s.slowLog)
		s.slowLogMu.Unlock()
		w.WriteInteger(int64(n))
	case "RESET":
		s.slowLogMu.Lock()
		s.slowLog = s.slowLog[:0]
		s.slowLogID = 0
		s.slowLogMu.Unlock()
		w.WriteSimpleString("OK")
	default:
		w.WriteError(fmt.Sprintf("unknown subcommand '%s' for SLOWLOG", sub))
	}
}

// SELECT command (for compatibility - we only have db0)
func (s *Server) cmdSelect(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'SELECT' command")
		return
	}

	db, err := strconv.Atoi(args[0].Str)
	if err != nil || db < 0 {
		w.WriteError("invalid DB index")
		return
	}

	if db != 0 {
		w.WriteError("invalid DB index")
		return
	}

	w.WriteSimpleString("OK")
}

// CLIENT command
func (s *Server) cmdClient(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'CLIENT' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "LIST":
		s.mu.RLock()
		var sb strings.Builder
		for _, c := range s.clients {
			age := int64(time.Since(c.createdAt).Seconds())
			idle := int64(time.Since(c.lastCommand).Seconds())
			fmt.Fprintf(&sb, "id=%d addr=%s age=%d idle=%d cmd=%d\n",
				c.id, c.addr, age, idle, c.cmdCount)
		}
		s.mu.RUnlock()
		w.WriteBulkString([]byte(sb.String()))

	case "GETNAME":
		w.WriteNull()

	case "SETNAME":
		w.WriteSimpleString("OK")

	case "ID":
		w.WriteInteger(client.id)

	case "INFO":
		age := int64(time.Since(client.createdAt).Seconds())
		idle := int64(time.Since(client.lastCommand).Seconds())
		info := fmt.Sprintf("id=%d addr=%s age=%d idle=%d cmd=%d",
			client.id, client.addr, age, idle, client.cmdCount)
		w.WriteBulkString([]byte(info))

	default:
		w.WriteError(fmt.Sprintf("Unknown subcommand '%s'", subCmd))
	}
}

// SCAN command
func (s *Server) cmdScan(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SCAN' command")
		return
	}

	cursor, err := strconv.Atoi(args[0].Str)
	if err != nil || cursor < 0 {
		w.WriteError("invalid cursor")
		return
	}

	// Parse options
	pattern := "*"
	count := 10

	for i := 1; i < len(args); i += 2 {
		if i+1 >= len(args) {
			w.WriteError("syntax error")
			return
		}
		opt := strings.ToUpper(args[i].Str)
		switch opt {
		case "MATCH":
			pattern = args[i+1].Str
		case "COUNT":
			c, err := strconv.Atoi(args[i+1].Str)
			if err != nil || c <= 0 {
				w.WriteError("invalid COUNT value")
				return
			}
			count = c
		}
	}

	// Get all keys, sort for stable cursor ordering
	allKeys := s.engine.Keys()
	sort.Strings(allKeys)

	// Apply pattern filter using filepath.Match for glob support
	var matched []string
	for _, key := range allKeys {
		if pattern == "*" {
			matched = append(matched, key)
		} else if ok, _ := filepath.Match(pattern, key); ok {
			matched = append(matched, key)
		}
	}

	// Paginate results
	start := cursor
	end := start + count
	if start >= len(matched) {
		// Return cursor 0 and empty array
		w.WriteRaw([]byte("*2\r\n$1\r\n0\r\n*0\r\n"))
		return
	}
	if end > len(matched) {
		end = len(matched)
	}

	nextCursor := end
	if nextCursor >= len(matched) {
		nextCursor = 0
	}

	// Write response: [nextCursor, [keys...]]
	result := matched[start:end]
	w.WriteRaw([]byte(fmt.Sprintf("*2\r\n$%d\r\n%d\r\n*%d\r\n",
		len(strconv.Itoa(nextCursor)), nextCursor, len(result))))
	for _, key := range result {
		w.WriteRaw([]byte(fmt.Sprintf("$%d\r\n%s\r\n", len(key), key)))
	}
}

// EXPIREAT command
func (s *Server) cmdExpireAt(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'EXPIREAT' command")
		return
	}

	timestamp, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("invalid timestamp")
		return
	}

	ttl := time.Until(time.Unix(timestamp, 0))
	if ttl <= 0 {
		// Already expired - delete the key
		s.engine.Delete(args[0].Str)
		w.WriteInteger(1)
		return
	}

	ok, err := s.engine.Expire(args[0].Str, ttl)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if ok {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

// PEXPIREAT command
func (s *Server) cmdPExpireAt(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'PEXPIREAT' command")
		return
	}

	timestamp, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("invalid timestamp")
		return
	}

	ttl := time.Until(time.UnixMilli(timestamp))
	if ttl <= 0 {
		s.engine.Delete(args[0].Str)
		w.WriteInteger(1)
		return
	}

	ok, err := s.engine.Expire(args[0].Str, ttl)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if ok {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

// RENAME command
func (s *Server) cmdRename(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'RENAME' command")
		return
	}

	oldKey := args[0].Str
	newKey := args[1].Str

	renamed, err := s.engine.Rename(oldKey, newKey, false)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if !renamed {
		w.WriteError("no such key")
		return
	}

	w.WriteSimpleString("OK")
}

// RENAMENX command
func (s *Server) cmdRenameNX(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'RENAMENX' command")
		return
	}

	oldKey := args[0].Str
	newKey := args[1].Str

	if !s.engine.Exists(oldKey) {
		w.WriteError("no such key")
		return
	}

	renamed, err := s.engine.Rename(oldKey, newKey, true)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if !renamed {
		w.WriteInteger(0)
		return
	}

	w.WriteInteger(1)
}

// RANDOMKEY command
func (s *Server) cmdRandomKey(w *protocol.Writer) {
	keys := s.engine.Keys()
	if len(keys) == 0 {
		w.WriteNull()
		return
	}
	w.WriteBulkString([]byte(keys[rand.Intn(len(keys))]))
}

// CONFIG command
func (s *Server) cmdConfig(w *protocol.Writer, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'CONFIG' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "GET":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'CONFIG GET'")
			return
		}
		param := strings.ToLower(args[1].Str)
		switch param {
		case "maxclients":
			w.WriteStringArray([]string{"maxclients", strconv.Itoa(s.config.MaxClients)})
		case "requirepass":
			if s.config.Password != "" {
				w.WriteStringArray([]string{"requirepass", "***"})
			} else {
				w.WriteStringArray([]string{"requirepass", ""})
			}
		default:
			w.WriteStringArray([]string{})
		}

	case "SET":
		// Read-only config for now
		w.WriteSimpleString("OK")

	case "RESETSTAT":
		w.WriteSimpleString("OK")

	default:
		w.WriteError(fmt.Sprintf("Unknown CONFIG subcommand '%s'", subCmd))
	}
}

// DEBUG command
func (s *Server) cmdDebug(w *protocol.Writer, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'DEBUG' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "SLEEP":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'DEBUG SLEEP'")
			return
		}
		seconds, err := strconv.ParseFloat(args[1].Str, 64)
		if err != nil {
			w.WriteError("invalid sleep time")
			return
		}
		time.Sleep(time.Duration(seconds * float64(time.Second)))
		w.WriteSimpleString("OK")

	default:
		w.WriteSimpleString("OK")
	}
}

// MEMORY command
func (s *Server) cmdMemory(w *protocol.Writer, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'MEMORY' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "USAGE":
		// Return approximate memory usage
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'MEMORY USAGE'")
			return
		}
		key := args[1].Str
		val, exists := s.engine.Get(key)
		if !exists {
			w.WriteNull()
			return
		}
		// Rough estimate: key length + value length + overhead
		usage := len(key) + len(val) + 64
		w.WriteInteger(int64(usage))

	case "STATS":
		var m runtime.MemStats
		runtime.ReadMemStats(&m)
		stats := fmt.Sprintf(`peak.allocated:%d
total.allocated:%d
startup.allocated:%d
keys.count:%d
keys.bytes-per-key:64
`,
			m.TotalAlloc,
			m.Alloc,
			m.Sys,
			s.engine.Size())
		w.WriteBulkString([]byte(stats))

	case "DOCTOR":
		w.WriteBulkString([]byte("Sam, I have no memory problems"))

	default:
		w.WriteError(fmt.Sprintf("Unknown MEMORY subcommand '%s'", subCmd))
	}
}

// LASTSAVE command
func (s *Server) cmdLastSave(w *protocol.Writer) {
	// Return current time as we persist immediately via WAL
	w.WriteInteger(time.Now().Unix())
}

// SAVE/BGSAVE command
func (s *Server) cmdSave(w *protocol.Writer) {
	// No-op as we use WAL for persistence
	w.WriteSimpleString("OK")
}

// Transaction commands

func (s *Server) cmdMulti(w *protocol.Writer, client *clientConn) {
	if client.inMulti {
		w.WriteError("MULTI calls can not be nested")
		return
	}
	client.inMulti = true
	client.multiQueue = make([]queuedCommand, 0)
	w.WriteSimpleString("OK")
}

func (s *Server) cmdExec(w *protocol.Writer, client *clientConn) {
	if !client.inMulti {
		w.WriteError("EXEC without MULTI")
		return
	}

	// Save the queue and reset transaction state BEFORE executing
	queue := client.multiQueue
	client.inMulti = false
	client.multiQueue = nil

	// Acquire engine-level lock for isolation
	s.engine.ExecLock()
	defer s.engine.ExecUnlock()

	// Execute all queued commands under the engine lock
	results := make([][]byte, len(queue))
	for i, qc := range queue {
		// Create a buffer to capture the response
		var buf strings.Builder
		tempWriter := protocol.NewWriter(&buf)
		s.executeCommand(tempWriter, client, qc.cmd, qc.args)
		results[i] = []byte(buf.String())
	}

	// Write the array of results
	w.WriteArrayHeader(len(results))
	for _, result := range results {
		w.WriteRaw(result)
	}
}

func (s *Server) cmdDiscard(w *protocol.Writer, client *clientConn) {
	if !client.inMulti {
		w.WriteError("DISCARD without MULTI")
		return
	}
	client.inMulti = false
	client.multiQueue = nil
	w.WriteSimpleString("OK")
}

// Pub/Sub commands

func (s *Server) cmdPublish(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'PUBLISH' command")
		return
	}

	channel := args[0].Str
	message := args[1].Str

	count := s.pubsub.Publish(channel, message)
	w.WriteInteger(int64(count))
}

func (s *Server) cmdSubscribe(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SUBSCRIBE' command")
		return
	}

	for _, arg := range args {
		channel := arg.Str
		s.pubsub.Subscribe(client, channel)
		client.subscriptions[channel] = true

		// Send subscribe confirmation
		w.WriteArrayHeader(3)
		w.WriteBulkString([]byte("subscribe"))
		w.WriteBulkString([]byte(channel))
		w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
	}
}

func (s *Server) cmdUnsubscribe(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) == 0 {
		// Unsubscribe from all channels
		for channel := range client.subscriptions {
			s.pubsub.Unsubscribe(client, channel)
			delete(client.subscriptions, channel)

			w.WriteArrayHeader(3)
			w.WriteBulkString([]byte("unsubscribe"))
			w.WriteBulkString([]byte(channel))
			w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
		}
		if len(client.subscriptions) == 0 {
			w.WriteArrayHeader(3)
			w.WriteBulkString([]byte("unsubscribe"))
			w.WriteNull()
			w.WriteInteger(0)
		}
		return
	}

	for _, arg := range args {
		channel := arg.Str
		s.pubsub.Unsubscribe(client, channel)
		delete(client.subscriptions, channel)

		w.WriteArrayHeader(3)
		w.WriteBulkString([]byte("unsubscribe"))
		w.WriteBulkString([]byte(channel))
		w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
	}
}

func (s *Server) cmdPSubscribe(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'PSUBSCRIBE' command")
		return
	}

	for _, arg := range args {
		pattern := arg.Str
		s.pubsub.PSubscribe(client, pattern)
		client.psubscriptions[pattern] = true

		w.WriteArrayHeader(3)
		w.WriteBulkString([]byte("psubscribe"))
		w.WriteBulkString([]byte(pattern))
		w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
	}
}

func (s *Server) cmdPUnsubscribe(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) == 0 {
		for pattern := range client.psubscriptions {
			s.pubsub.PUnsubscribe(client, pattern)
			delete(client.psubscriptions, pattern)

			w.WriteArrayHeader(3)
			w.WriteBulkString([]byte("punsubscribe"))
			w.WriteBulkString([]byte(pattern))
			w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
		}
		if len(client.psubscriptions) == 0 {
			w.WriteArrayHeader(3)
			w.WriteBulkString([]byte("punsubscribe"))
			w.WriteNull()
			w.WriteInteger(0)
		}
		return
	}

	for _, arg := range args {
		pattern := arg.Str
		s.pubsub.PUnsubscribe(client, pattern)
		delete(client.psubscriptions, pattern)

		w.WriteArrayHeader(3)
		w.WriteBulkString([]byte("punsubscribe"))
		w.WriteBulkString([]byte(pattern))
		w.WriteInteger(int64(len(client.subscriptions) + len(client.psubscriptions)))
	}
}

func (s *Server) cmdPubSub(w *protocol.Writer, args []protocol.Value) {
	if len(args) == 0 {
		w.WriteError("wrong number of arguments for 'PUBSUB' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "CHANNELS":
		pattern := "*"
		if len(args) > 1 {
			pattern = args[1].Str
		}
		channels := s.pubsub.Channels(pattern)
		w.WriteStringArray(channels)

	case "NUMSUB":
		if len(args) < 2 {
			w.WriteStringArray([]string{})
			return
		}
		result := make([]string, 0, (len(args)-1)*2)
		for _, arg := range args[1:] {
			channel := arg.Str
			count := s.pubsub.NumSub(channel)
			result = append(result, channel, strconv.Itoa(count))
		}
		w.WriteStringArray(result)

	case "NUMPAT":
		count := s.pubsub.NumPat()
		w.WriteInteger(int64(count))

	default:
		w.WriteError(fmt.Sprintf("Unknown PUBSUB subcommand '%s'", subCmd))
	}
}

// PubSub methods

func (ps *PubSub) Subscribe(client *clientConn, channel string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.channels[channel] == nil {
		ps.channels[channel] = make(map[*clientConn]bool)
	}
	ps.channels[channel][client] = true
}

func (ps *PubSub) Unsubscribe(client *clientConn, channel string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.channels[channel] != nil {
		delete(ps.channels[channel], client)
		if len(ps.channels[channel]) == 0 {
			delete(ps.channels, channel)
		}
	}
}

func (ps *PubSub) PSubscribe(client *clientConn, pattern string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.patterns[pattern] == nil {
		ps.patterns[pattern] = make(map[*clientConn]bool)
	}
	ps.patterns[pattern][client] = true
}

func (ps *PubSub) PUnsubscribe(client *clientConn, pattern string) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	if ps.patterns[pattern] != nil {
		delete(ps.patterns[pattern], client)
		if len(ps.patterns[pattern]) == 0 {
			delete(ps.patterns, pattern)
		}
	}
}

func (ps *PubSub) Publish(channel string, message string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0

	// Send to direct subscribers
	if subscribers, ok := ps.channels[channel]; ok {
		for client := range subscribers {
			ps.sendMessage(client, "message", channel, message)
			count++
		}
	}

	// Send to pattern subscribers
	for pattern, subscribers := range ps.patterns {
		if ps.matchPattern(pattern, channel) {
			for client := range subscribers {
				ps.sendPMessage(client, pattern, channel, message)
				count++
			}
		}
	}

	return count
}

func (ps *PubSub) sendMessage(client *clientConn, msgType, channel, message string) {
	var buf bytes.Buffer
	w := protocol.NewWriter(&buf)
	w.WriteArrayHeader(3)
	w.WriteBulkString([]byte(msgType))
	w.WriteBulkString([]byte(channel))
	w.WriteBulkString([]byte(message))

	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	_ = writeAll(client.conn, buf.Bytes())
}

func (ps *PubSub) sendPMessage(client *clientConn, pattern, channel, message string) {
	var buf bytes.Buffer
	w := protocol.NewWriter(&buf)
	w.WriteArrayHeader(4)
	w.WriteBulkString([]byte("pmessage"))
	w.WriteBulkString([]byte(pattern))
	w.WriteBulkString([]byte(channel))
	w.WriteBulkString([]byte(message))

	client.writeMu.Lock()
	defer client.writeMu.Unlock()
	_ = writeAll(client.conn, buf.Bytes())
}

func writeAll(conn net.Conn, data []byte) error {
	for len(data) > 0 {
		n, err := conn.Write(data)
		if err != nil {
			return err
		}
		data = data[n:]
	}
	return nil
}

func (ps *PubSub) matchPattern(pattern, channel string) bool {
	// Simple glob pattern matching
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

	matched, _ := regexp.MatchString(regexStr, channel)
	return matched
}

func (ps *PubSub) Channels(pattern string) []string {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	result := make([]string, 0)
	for channel := range ps.channels {
		if pattern == "*" || ps.matchPattern(pattern, channel) {
			result = append(result, channel)
		}
	}
	return result
}

func (ps *PubSub) NumSub(channel string) int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	if subscribers, ok := ps.channels[channel]; ok {
		return len(subscribers)
	}
	return 0
}

func (ps *PubSub) NumPat() int {
	ps.mu.RLock()
	defer ps.mu.RUnlock()

	count := 0
	for _, subscribers := range ps.patterns {
		count += len(subscribers)
	}
	return count
}

func (ps *PubSub) UnsubscribeAll(client *clientConn) {
	ps.mu.Lock()
	defer ps.mu.Unlock()

	// Remove from all channels
	for channel, subscribers := range ps.channels {
		delete(subscribers, client)
		if len(subscribers) == 0 {
			delete(ps.channels, channel)
		}
	}

	// Remove from all patterns
	for pattern, subscribers := range ps.patterns {
		delete(subscribers, client)
		if len(subscribers) == 0 {
			delete(ps.patterns, pattern)
		}
	}
}

// Additional String commands

func (s *Server) cmdGetSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'GETSET' command")
		return
	}

	key := args[0].Str
	newValue := []byte(args[1].Str)

	// Get old value
	oldValue, exists := s.engine.Get(key)

	// Set new value
	if err := s.engine.Set(key, newValue); err != nil {
		w.WriteError("internal error")
		return
	}

	if exists {
		w.WriteBulkString(oldValue)
	} else {
		w.WriteNull()
	}
}

func (s *Server) cmdGetEx(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'GETEX' command")
		return
	}

	key := args[0].Str
	value, exists := s.engine.Get(key)

	if !exists {
		w.WriteNull()
		return
	}

	action := ""
	var ttl time.Duration

	// Parse options
	for i := 1; i < len(args); i++ {
		opt := strings.ToUpper(args[i].Str)
		switch opt {
		case "EX":
			if i+1 >= len(args) || action != "" {
				w.WriteError("syntax error")
				return
			}
			seconds, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil || seconds <= 0 {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Duration(seconds) * time.Second
			action = "expire"
			i++
		case "PX":
			if i+1 >= len(args) || action != "" {
				w.WriteError("syntax error")
				return
			}
			millis, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil || millis <= 0 {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Duration(millis) * time.Millisecond
			action = "expire"
			i++
		case "EXAT":
			if i+1 >= len(args) || action != "" {
				w.WriteError("syntax error")
				return
			}
			timestamp, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Until(time.Unix(timestamp, 0))
			if ttl <= 0 {
				action = "delete"
			} else {
				action = "expire"
			}
			i++
		case "PXAT":
			if i+1 >= len(args) || action != "" {
				w.WriteError("syntax error")
				return
			}
			timestamp, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil {
				w.WriteError("invalid expire time")
				return
			}
			ttl = time.Until(time.UnixMilli(timestamp))
			if ttl <= 0 {
				action = "delete"
			} else {
				action = "expire"
			}
			i++
		case "PERSIST":
			if action != "" {
				w.WriteError("syntax error")
				return
			}
			action = "persist"
		default:
			w.WriteError("syntax error")
			return
		}
	}

	switch action {
	case "expire":
		if _, err := s.engine.Expire(key, ttl); err != nil {
			w.WriteError("internal error")
			return
		}
	case "persist":
		if _, err := s.engine.Persist(key); err != nil {
			w.WriteError("internal error")
			return
		}
	case "delete":
		if _, err := s.engine.Delete(key); err != nil {
			w.WriteError("internal error")
			return
		}
	}

	w.WriteBulkString(value)
}

func (s *Server) cmdGetDel(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'GETDEL' command")
		return
	}

	key := args[0].Str
	value, exists := s.engine.Get(key)

	if !exists {
		w.WriteNull()
		return
	}

	s.engine.Delete(key)
	w.WriteBulkString(value)
}

func (s *Server) cmdGetRange(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'GETRANGE' command")
		return
	}

	key := args[0].Str
	start, err1 := strconv.Atoi(args[1].Str)
	end, err2 := strconv.Atoi(args[2].Str)

	if err1 != nil || err2 != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}

	value, exists := s.engine.Get(key)
	if !exists {
		w.WriteBulkString([]byte{})
		return
	}

	length := len(value)
	if length == 0 {
		w.WriteBulkString([]byte{})
		return
	}

	// Handle negative indices
	if start < 0 {
		start = length + start
	}
	if end < 0 {
		end = length + end
	}

	// Clamp indices
	if start < 0 {
		start = 0
	}
	if end >= length {
		end = length - 1
	}

	if start > end || start >= length {
		w.WriteBulkString([]byte{})
		return
	}

	w.WriteBulkString(value[start : end+1])
}

func (s *Server) cmdSetRange(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'SETRANGE' command")
		return
	}

	key := args[0].Str
	offset, err := strconv.Atoi(args[1].Str)
	if err != nil || offset < 0 {
		w.WriteError("offset is out of range")
		return
	}

	replacement := []byte(args[2].Str)

	value, _ := s.engine.Get(key)

	// Extend value if necessary
	neededLen := offset + len(replacement)
	if neededLen > len(value) {
		newValue := make([]byte, neededLen)
		copy(newValue, value)
		value = newValue
	}

	// Apply replacement
	copy(value[offset:], replacement)

	if err := s.engine.Set(key, value); err != nil {
		w.WriteError("internal error")
		return
	}

	w.WriteInteger(int64(len(value)))
}

func (s *Server) cmdMSetNX(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 || len(args)%2 != 0 {
		w.WriteError("wrong number of arguments for 'MSETNX' command")
		return
	}

	pairs := make(map[string][]byte, len(args)/2)
	for i := 0; i < len(args); i += 2 {
		pairs[args[i].Str] = []byte(args[i+1].Str)
	}

	ok, err := s.engine.MSetNX(pairs)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if ok {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdIncrByFloat(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'INCRBYFLOAT' command")
		return
	}

	key := args[0].Str
	increment, err := strconv.ParseFloat(args[1].Str, 64)
	if err != nil {
		w.WriteError("value is not a valid float")
		return
	}

	newValue, err := s.engine.IncrByFloat(key, increment)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	newStr := strconv.FormatFloat(newValue, 'f', -1, 64)
	w.WriteBulkString([]byte(newStr))
}

// Additional Key commands

func (s *Server) cmdTouch(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'TOUCH' command")
		return
	}

	count := 0
	for _, arg := range args {
		if s.engine.Exists(arg.Str) {
			count++
		}
	}

	w.WriteInteger(int64(count))
}

func (s *Server) cmdObject(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'OBJECT' command")
		return
	}

	subCmd := strings.ToUpper(args[0].Str)
	switch subCmd {
	case "ENCODING":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'OBJECT ENCODING'")
			return
		}
		key := args[1].Str
		if !s.engine.Exists(key) {
			w.WriteNull()
			return
		}
		// FlashDB stores everything as raw bytes
		w.WriteBulkString([]byte("raw"))

	case "FREQ":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'OBJECT FREQ'")
			return
		}
		// Return 0 as we don't track access frequency
		w.WriteInteger(0)

	case "IDLETIME":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'OBJECT IDLETIME'")
			return
		}
		// Return 0 as we don't track idle time
		w.WriteInteger(0)

	case "REFCOUNT":
		if len(args) != 2 {
			w.WriteError("wrong number of arguments for 'OBJECT REFCOUNT'")
			return
		}
		if !s.engine.Exists(args[1].Str) {
			w.WriteNull()
			return
		}
		// Always 1 in FlashDB
		w.WriteInteger(1)

	case "HELP":
		help := []string{
			"OBJECT ENCODING <key> - Return the encoding of the object stored at <key>.",
			"OBJECT FREQ <key> - Return the access frequency index of the object stored at <key>.",
			"OBJECT IDLETIME <key> - Return the idle time of the object stored at <key>.",
			"OBJECT REFCOUNT <key> - Return the reference count of the object stored at <key>.",
		}
		w.WriteStringArray(help)

	default:
		w.WriteError(fmt.Sprintf("Unknown OBJECT subcommand '%s'", subCmd))
	}
}

func (s *Server) cmdDump(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'DUMP' command")
		return
	}

	key := args[0].Str
	value, exists := s.engine.Get(key)
	if !exists {
		w.WriteNull()
		return
	}

	// Return raw value (simplified dump - real Redis uses RDB format)
	w.WriteBulkString(value)
}

func (s *Server) cmdCopy(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'COPY' command")
		return
	}

	source := args[0].Str
	dest := args[1].Str
	replace := false

	// Parse options
	for i := 2; i < len(args); i++ {
		opt := strings.ToUpper(args[i].Str)
		if opt == "REPLACE" {
			replace = true
		}
	}

	copied, err := s.engine.Copy(source, dest, replace)
	if err != nil {
		w.WriteError("internal error")
		return
	}

	if copied {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

// ========================
// Sorted Set Commands
// ========================

func (s *Server) cmdZAdd(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'ZADD' command")
		return
	}

	key := args[0].Str
	var members []store.ScoredMember

	// Parse score-member pairs (must have even number after key)
	for i := 1; i+1 < len(args); i += 2 {
		score, err := strconv.ParseFloat(args[i].Str, 64)
		if err != nil {
			w.WriteError("value is not a valid float")
			return
		}
		members = append(members, store.ScoredMember{
			Score:  score,
			Member: args[i+1].Str,
		})
	}

	if len(members) == 0 {
		w.WriteError("wrong number of arguments for 'ZADD' command")
		return
	}

	added, err := s.engine.ZAdd(key, members...)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZADD error: %v", err)
		return
	}
	w.WriteInteger(int64(added))
}

func (s *Server) cmdZScore(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'ZSCORE' command")
		return
	}

	key := args[0].Str
	member := args[1].Str

	score, exists := s.engine.ZScore(key, member)
	if !exists {
		w.WriteNull()
		return
	}
	w.WriteBulkString([]byte(strconv.FormatFloat(score, 'f', -1, 64)))
}

func (s *Server) cmdZRem(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'ZREM' command")
		return
	}

	key := args[0].Str
	members := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		members[i-1] = args[i].Str
	}

	removed, err := s.engine.ZRem(key, members...)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZREM error: %v", err)
		return
	}
	w.WriteInteger(int64(removed))
}

func (s *Server) cmdZCard(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'ZCARD' command")
		return
	}

	card := s.engine.ZCard(args[0].Str)
	w.WriteInteger(int64(card))
}

func (s *Server) cmdZRank(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'ZRANK' command")
		return
	}

	rank, exists := s.engine.ZRank(args[0].Str, args[1].Str)
	if !exists {
		w.WriteNull()
		return
	}
	w.WriteInteger(int64(rank))
}

func (s *Server) cmdZRevRank(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'ZREVRANK' command")
		return
	}

	rank, exists := s.engine.ZRevRank(args[0].Str, args[1].Str)
	if !exists {
		w.WriteNull()
		return
	}
	w.WriteInteger(int64(rank))
}

func (s *Server) cmdZRange(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'ZRANGE' command")
		return
	}

	key := args[0].Str
	start, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	stop, err := strconv.Atoi(args[2].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}

	withScores := false
	if len(args) > 3 && strings.ToUpper(args[3].Str) == "WITHSCORES" {
		withScores = true
	}

	members := s.engine.ZRange(key, start, stop, withScores)
	s.writeZRangeResult(w, members, withScores)
}

func (s *Server) cmdZRevRange(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'ZREVRANGE' command")
		return
	}

	key := args[0].Str
	start, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	stop, err := strconv.Atoi(args[2].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}

	withScores := false
	if len(args) > 3 && strings.ToUpper(args[3].Str) == "WITHSCORES" {
		withScores = true
	}

	members := s.engine.ZRevRange(key, start, stop, withScores)
	s.writeZRangeResult(w, members, withScores)
}

func (s *Server) cmdZRangeByScore(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'ZRANGEBYSCORE' command")
		return
	}

	key := args[0].Str
	min, err := parseZSetScore(args[1].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}
	max, err := parseZSetScore(args[2].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}

	withScores := false
	offset := 0
	count := -1

	for i := 3; i < len(args); i++ {
		opt := strings.ToUpper(args[i].Str)
		if opt == "WITHSCORES" {
			withScores = true
		} else if opt == "LIMIT" && i+2 < len(args) {
			offset, _ = strconv.Atoi(args[i+1].Str)
			count, _ = strconv.Atoi(args[i+2].Str)
			i += 2
		}
	}

	members := s.engine.ZRangeByScore(key, min, max, withScores, offset, count)
	s.writeZRangeResult(w, members, withScores)
}

func (s *Server) cmdZRevRangeByScore(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'ZREVRANGEBYSCORE' command")
		return
	}

	key := args[0].Str
	max, err := parseZSetScore(args[1].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}
	min, err := parseZSetScore(args[2].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}

	withScores := false
	offset := 0
	count := -1

	for i := 3; i < len(args); i++ {
		opt := strings.ToUpper(args[i].Str)
		if opt == "WITHSCORES" {
			withScores = true
		} else if opt == "LIMIT" && i+2 < len(args) {
			offset, _ = strconv.Atoi(args[i+1].Str)
			count, _ = strconv.Atoi(args[i+2].Str)
			i += 2
		}
	}

	members := s.engine.ZRevRangeByScore(key, max, min, withScores, offset, count)
	s.writeZRangeResult(w, members, withScores)
}

func (s *Server) cmdZCount(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'ZCOUNT' command")
		return
	}

	key := args[0].Str
	min, err := parseZSetScore(args[1].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}
	max, err := parseZSetScore(args[2].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}

	count := s.engine.ZCount(key, min, max)
	w.WriteInteger(int64(count))
}

func (s *Server) cmdZIncrBy(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'ZINCRBY' command")
		return
	}

	key := args[0].Str
	increment, err := strconv.ParseFloat(args[1].Str, 64)
	if err != nil {
		w.WriteError("value is not a valid float")
		return
	}
	member := args[2].Str

	newScore, err := s.engine.ZIncrBy(key, member, increment)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZINCRBY error: %v", err)
		return
	}
	w.WriteBulkString([]byte(strconv.FormatFloat(newScore, 'f', -1, 64)))
}

func (s *Server) cmdZRemRangeByRank(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'ZREMRANGEBYRANK' command")
		return
	}

	key := args[0].Str
	start, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	stop, err := strconv.Atoi(args[2].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}

	removed, err := s.engine.ZRemRangeByRank(key, start, stop)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZREMRANGEBYRANK error: %v", err)
		return
	}
	w.WriteInteger(int64(removed))
}

func (s *Server) cmdZRemRangeByScore(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'ZREMRANGEBYSCORE' command")
		return
	}

	key := args[0].Str
	min, err := parseZSetScore(args[1].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}
	max, err := parseZSetScore(args[2].Str)
	if err != nil {
		w.WriteError("min or max is not a float")
		return
	}

	removed, err := s.engine.ZRemRangeByScore(key, min, max)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZREMRANGEBYSCORE error: %v", err)
		return
	}
	w.WriteInteger(int64(removed))
}

func (s *Server) cmdZPopMin(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'ZPOPMIN' command")
		return
	}

	key := args[0].Str
	count := 1
	if len(args) > 1 {
		var err error
		count, err = strconv.Atoi(args[1].Str)
		if err != nil || count < 0 {
			w.WriteError("value is not an integer or out of range")
			return
		}
	}

	members, err := s.engine.ZPopMin(key, count)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZPOPMIN error: %v", err)
		return
	}
	s.writeZRangeResult(w, members, true)
}

func (s *Server) cmdZPopMax(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'ZPOPMAX' command")
		return
	}

	key := args[0].Str
	count := 1
	if len(args) > 1 {
		var err error
		count, err = strconv.Atoi(args[1].Str)
		if err != nil || count < 0 {
			w.WriteError("value is not an integer or out of range")
			return
		}
	}

	members, err := s.engine.ZPopMax(key, count)
	if err != nil {
		w.WriteError("internal error")
		log.Printf("server: ZPOPMAX error: %v", err)
		return
	}
	s.writeZRangeResult(w, members, true)
}

// Helper functions for sorted sets

func (s *Server) writeZRangeResult(w *protocol.Writer, members []store.ScoredMember, withScores bool) {
	if members == nil {
		w.WriteArrayHeader(0)
		return
	}

	if withScores {
		w.WriteArrayHeader(len(members) * 2)
		for _, m := range members {
			w.WriteBulkString([]byte(m.Member))
			w.WriteBulkString([]byte(strconv.FormatFloat(m.Score, 'f', -1, 64)))
		}
	} else {
		w.WriteArrayHeader(len(members))
		for _, m := range members {
			w.WriteBulkString([]byte(m.Member))
		}
	}
}

func parseZSetScore(s string) (float64, error) {
	switch strings.ToLower(s) {
	case "-inf":
		return math.Inf(-1), nil
	case "+inf", "inf":
		return math.Inf(1), nil
	default:
		return strconv.ParseFloat(s, 64)
	}
}

// ─── Hash commands ──────────────────────────────────────────────────────────

func (s *Server) cmdHSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 || len(args)%2 == 0 {
		w.WriteError("wrong number of arguments for 'HSET' command")
		return
	}
	key := args[0].Str
	fields := make([]store.HashFieldValue, 0, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		fields = append(fields, store.HashFieldValue{Field: args[i].Str, Value: []byte(args[i+1].Str)})
	}
	n, err := s.engine.HSet(key, fields...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdHGet(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'HGET' command")
		return
	}
	val, ok := s.engine.HGet(args[0].Str, args[1].Str)
	if !ok {
		w.WriteNull()
		return
	}
	w.WriteBulkString(val)
}

func (s *Server) cmdHMSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 3 || len(args)%2 == 0 {
		w.WriteError("wrong number of arguments for 'HMSET' command")
		return
	}
	key := args[0].Str
	fields := make([]store.HashFieldValue, 0, (len(args)-1)/2)
	for i := 1; i < len(args); i += 2 {
		fields = append(fields, store.HashFieldValue{Field: args[i].Str, Value: []byte(args[i+1].Str)})
	}
	if _, err := s.engine.HSet(key, fields...); err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteSimpleString("OK")
}

func (s *Server) cmdHMGet(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'HMGET' command")
		return
	}
	key := args[0].Str
	results := make([][]byte, len(args)-1)
	nulls := make([]bool, len(args)-1)
	for i := 1; i < len(args); i++ {
		val, ok := s.engine.HGet(key, args[i].Str)
		if ok {
			results[i-1] = val
		} else {
			nulls[i-1] = true
		}
	}
	w.WriteArrayWithNulls(results, nulls)
}

func (s *Server) cmdHDel(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'HDEL' command")
		return
	}
	key := args[0].Str
	fields := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		fields[i-1] = args[i].Str
	}
	n, err := s.engine.HDel(key, fields...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdHExists(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'HEXISTS' command")
		return
	}
	if s.engine.HExists(args[0].Str, args[1].Str) {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdHLen(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'HLEN' command")
		return
	}
	w.WriteInteger(int64(s.engine.HLen(args[0].Str)))
}

func (s *Server) cmdHGetAll(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'HGETALL' command")
		return
	}
	pairs := s.engine.HGetAll(args[0].Str)
	w.WriteArrayHeader(len(pairs) * 2)
	for _, p := range pairs {
		w.WriteBulkString([]byte(p.Field))
		w.WriteBulkString(p.Value)
	}
}

func (s *Server) cmdHKeys(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'HKEYS' command")
		return
	}
	w.WriteStringArray(s.engine.HKeys(args[0].Str))
}

func (s *Server) cmdHVals(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'HVALS' command")
		return
	}
	w.WriteArray(s.engine.HVals(args[0].Str))
}

func (s *Server) cmdHIncrBy(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'HINCRBY' command")
		return
	}
	delta, err := strconv.ParseInt(args[2].Str, 10, 64)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	result, err := s.engine.HIncrBy(args[0].Str, args[1].Str, delta)
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteInteger(result)
}

func (s *Server) cmdHIncrByFloat(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'HINCRBYFLOAT' command")
		return
	}
	delta, err := strconv.ParseFloat(args[2].Str, 64)
	if err != nil {
		w.WriteError("value is not a valid float")
		return
	}
	result, err := s.engine.HIncrByFloat(args[0].Str, args[1].Str, delta)
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteBulkString([]byte(strconv.FormatFloat(result, 'f', -1, 64)))
}

func (s *Server) cmdHSetNX(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'HSETNX' command")
		return
	}
	ok, err := s.engine.HSetNX(args[0].Str, args[1].Str, []byte(args[2].Str))
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if ok {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

// ─── List commands ──────────────────────────────────────────────────────────

func (s *Server) cmdLPush(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'LPUSH' command")
		return
	}
	key := args[0].Str
	values := make([][]byte, len(args)-1)
	for i := 1; i < len(args); i++ {
		values[i-1] = []byte(args[i].Str)
	}
	n, err := s.engine.LPush(key, values...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdRPush(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'RPUSH' command")
		return
	}
	key := args[0].Str
	values := make([][]byte, len(args)-1)
	for i := 1; i < len(args); i++ {
		values[i-1] = []byte(args[i].Str)
	}
	n, err := s.engine.RPush(key, values...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdLPop(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'LPOP' command")
		return
	}
	val, ok, err := s.engine.LPop(args[0].Str)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if !ok {
		w.WriteNull()
		return
	}
	w.WriteBulkString(val)
}

func (s *Server) cmdRPop(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'RPOP' command")
		return
	}
	val, ok, err := s.engine.RPop(args[0].Str)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if !ok {
		w.WriteNull()
		return
	}
	w.WriteBulkString(val)
}

func (s *Server) cmdLLen(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'LLEN' command")
		return
	}
	w.WriteInteger(int64(s.engine.LLen(args[0].Str)))
}

func (s *Server) cmdLIndex(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'LINDEX' command")
		return
	}
	index, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	val, ok := s.engine.LIndex(args[0].Str, index)
	if !ok {
		w.WriteNull()
		return
	}
	w.WriteBulkString(val)
}

func (s *Server) cmdLSet(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'LSET' command")
		return
	}
	index, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	if err := s.engine.LSet(args[0].Str, index, []byte(args[2].Str)); err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteSimpleString("OK")
}

func (s *Server) cmdLRange(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'LRANGE' command")
		return
	}
	start, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	stop, err := strconv.Atoi(args[2].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	items := s.engine.LRange(args[0].Str, start, stop)
	w.WriteArray(items)
}

func (s *Server) cmdLInsert(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 4 {
		w.WriteError("wrong number of arguments for 'LINSERT' command")
		return
	}
	pos := strings.ToUpper(args[1].Str)
	var before bool
	switch pos {
	case "BEFORE":
		before = true
	case "AFTER":
		before = false
	default:
		w.WriteError("syntax error")
		return
	}
	n, err := s.engine.LInsert(args[0].Str, before, []byte(args[2].Str), []byte(args[3].Str))
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdLRem(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'LREM' command")
		return
	}
	count, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	n, err := s.engine.LRem(args[0].Str, count, []byte(args[2].Str))
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdLTrim(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'LTRIM' command")
		return
	}
	start, err := strconv.Atoi(args[1].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	stop, err := strconv.Atoi(args[2].Str)
	if err != nil {
		w.WriteError("value is not an integer or out of range")
		return
	}
	if err := s.engine.LTrim(args[0].Str, start, stop); err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteSimpleString("OK")
}

// ─── Set commands ───────────────────────────────────────────────────────────

func (s *Server) cmdSAdd(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'SADD' command")
		return
	}
	key := args[0].Str
	members := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		members[i-1] = args[i].Str
	}
	n, err := s.engine.SAdd(key, members...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdSRem(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 2 {
		w.WriteError("wrong number of arguments for 'SREM' command")
		return
	}
	key := args[0].Str
	members := make([]string, len(args)-1)
	for i := 1; i < len(args); i++ {
		members[i-1] = args[i].Str
	}
	n, err := s.engine.SRem(key, members...)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	w.WriteInteger(int64(n))
}

func (s *Server) cmdSIsMember(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 2 {
		w.WriteError("wrong number of arguments for 'SISMEMBER' command")
		return
	}
	if s.engine.SIsMember(args[0].Str, args[1].Str) {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdSCard(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'SCARD' command")
		return
	}
	w.WriteInteger(int64(s.engine.SCard(args[0].Str)))
}

func (s *Server) cmdSMembers(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'SMEMBERS' command")
		return
	}
	w.WriteStringArray(s.engine.SMembers(args[0].Str))
}

func (s *Server) cmdSRandMember(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 || len(args) > 2 {
		w.WriteError("wrong number of arguments for 'SRANDMEMBER' command")
		return
	}
	count := 1
	if len(args) == 2 {
		var err error
		count, err = strconv.Atoi(args[1].Str)
		if err != nil {
			w.WriteError("value is not an integer or out of range")
			return
		}
	}
	members := s.engine.SRandMember(args[0].Str, count)
	if len(args) == 1 {
		// Single element mode: return bulk string or nil
		if len(members) == 0 {
			w.WriteNull()
		} else {
			w.WriteBulkString([]byte(members[0]))
		}
	} else {
		w.WriteStringArray(members)
	}
}

func (s *Server) cmdSPop(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 || len(args) > 2 {
		w.WriteError("wrong number of arguments for 'SPOP' command")
		return
	}
	count := 1
	if len(args) == 2 {
		var err error
		count, err = strconv.Atoi(args[1].Str)
		if err != nil {
			w.WriteError("value is not an integer or out of range")
			return
		}
		if count < 0 {
			w.WriteError("value is not an integer or out of range")
			return
		}
	}
	members, err := s.engine.SPop(args[0].Str, count)
	if err != nil {
		w.WriteError("internal error")
		return
	}
	if len(args) == 1 {
		// Single element mode: return bulk string or nil
		if len(members) == 0 {
			w.WriteNull()
		} else {
			w.WriteBulkString([]byte(members[0]))
		}
	} else {
		w.WriteStringArray(members)
	}
}

func (s *Server) cmdSInter(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SINTER' command")
		return
	}
	keys := make([]string, len(args))
	for i, a := range args {
		keys[i] = a.Str
	}
	w.WriteStringArray(s.engine.SInter(keys...))
}

func (s *Server) cmdSUnion(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SUNION' command")
		return
	}
	keys := make([]string, len(args))
	for i, a := range args {
		keys[i] = a.Str
	}
	w.WriteStringArray(s.engine.SUnion(keys...))
}

func (s *Server) cmdSDiff(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SDIFF' command")
		return
	}
	keys := make([]string, len(args))
	for i, a := range args {
		keys[i] = a.Str
	}
	w.WriteStringArray(s.engine.SDiff(keys...))
}

// ========================
// Phase 6: Time-Series Commands
// ========================

func (s *Server) cmdTSAdd(w *protocol.Writer, args []protocol.Value) {
	// TS.ADD key timestamp value [RETENTION ms]
	if len(args) < 3 {
		w.WriteError("wrong number of arguments for 'TS.ADD' command")
		return
	}
	key := args[0].Str

	var ts int64
	if args[1].Str == "*" {
		ts = 0 // auto-timestamp
	} else {
		var err error
		ts, err = strconv.ParseInt(args[1].Str, 10, 64)
		if err != nil {
			w.WriteError("ERR invalid timestamp")
			return
		}
	}

	val, err := strconv.ParseFloat(args[2].Str, 64)
	if err != nil {
		w.WriteError("ERR invalid value (must be float)")
		return
	}

	var retention time.Duration
	for i := 3; i < len(args)-1; i++ {
		if strings.ToUpper(args[i].Str) == "RETENTION" {
			ms, err := strconv.ParseInt(args[i+1].Str, 10, 64)
			if err != nil {
				w.WriteError("ERR invalid retention value")
				return
			}
			retention = time.Duration(ms) * time.Millisecond
		}
	}

	inserted, err := s.engine.TSAdd(key, ts, val, retention)
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteInteger(inserted)
}

func (s *Server) cmdTSGet(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'TS.GET' command")
		return
	}
	dp, ok := s.engine.TSGet(args[0].Str)
	if !ok {
		w.WriteNull()
		return
	}
	w.WriteArrayHeader(2)
	w.WriteInteger(dp.Timestamp)
	w.WriteBulkString([]byte(strconv.FormatFloat(dp.Value, 'f', -1, 64)))
}

func (s *Server) cmdTSRange(w *protocol.Writer, args []protocol.Value) {
	// TS.RANGE key fromTS toTS
	if len(args) != 3 {
		w.WriteError("wrong number of arguments for 'TS.RANGE' command")
		return
	}
	fromTS, err := strconv.ParseInt(args[1].Str, 10, 64)
	if err != nil {
		w.WriteError("ERR invalid from timestamp")
		return
	}
	toTS, err := strconv.ParseInt(args[2].Str, 10, 64)
	if err != nil {
		w.WriteError("ERR invalid to timestamp")
		return
	}

	points, err := s.engine.TSRange(args[0].Str, fromTS, toTS)
	if err != nil {
		w.WriteError(err.Error())
		return
	}

	w.WriteArrayHeader(len(points))
	for _, p := range points {
		w.WriteArrayHeader(2)
		w.WriteInteger(p.Timestamp)
		w.WriteBulkString([]byte(strconv.FormatFloat(p.Value, 'f', -1, 64)))
	}
}

func (s *Server) cmdTSInfo(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'TS.INFO' command")
		return
	}
	info, err := s.engine.TSInfo(args[0].Str)
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	w.WriteArrayHeader(10)
	w.WriteBulkString([]byte("totalSamples"))
	w.WriteInteger(int64(info.TotalSamples))
	w.WriteBulkString([]byte("firstTimestamp"))
	w.WriteInteger(info.FirstTS)
	w.WriteBulkString([]byte("lastTimestamp"))
	w.WriteInteger(info.LastTS)
	w.WriteBulkString([]byte("retentionMs"))
	w.WriteInteger(info.Retention.Milliseconds())
	w.WriteBulkString([]byte("memoryBytes"))
	w.WriteInteger(int64(info.MemoryBytes))
}

func (s *Server) cmdTSDel(w *protocol.Writer, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'TS.DEL' command")
		return
	}
	ok, err := s.engine.TSDel(args[0].Str)
	if err != nil {
		w.WriteError(err.Error())
		return
	}
	if ok {
		w.WriteInteger(1)
	} else {
		w.WriteInteger(0)
	}
}

func (s *Server) cmdTSKeys(w *protocol.Writer) {
	w.WriteStringArray(s.engine.TSKeys())
}

// ========================
// Phase 6: Hot Key Detection
// ========================

func (s *Server) cmdHotKeys(w *protocol.Writer, args []protocol.Value) {
	n := 10
	if len(args) >= 1 {
		var err error
		n, err = strconv.Atoi(args[0].Str)
		if err != nil || n <= 0 {
			w.WriteError("ERR invalid count")
			return
		}
	}
	entries := s.engine.HotKeys(n)
	w.WriteArrayHeader(len(entries) * 2)
	for _, e := range entries {
		w.WriteBulkString([]byte(e.Key))
		w.WriteInteger(e.Count)
	}
}

// ========================
// Phase 6: Snapshot Commands
// ========================

func (s *Server) cmdSnapshot(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'SNAPSHOT' command")
		return
	}
	sub := strings.ToUpper(args[0].Str)
	switch sub {
	case "CREATE":
		id := ""
		if len(args) >= 2 {
			id = args[1].Str
		}
		meta, err := s.engine.SnapshotCreate(id)
		if err != nil {
			w.WriteError(err.Error())
			return
		}
		w.WriteArrayHeader(4)
		w.WriteBulkString([]byte("id"))
		w.WriteBulkString([]byte(meta.ID))
	case "LIST":
		metas, err := s.engine.SnapshotList()
		if err != nil {
			w.WriteError(err.Error())
			return
		}
		w.WriteArrayHeader(len(metas))
		for _, m := range metas {
			w.WriteArrayHeader(6)
			w.WriteBulkString([]byte("id"))
			w.WriteBulkString([]byte(m.ID))
			w.WriteBulkString([]byte("size"))
			w.WriteInteger(m.SizeBytes)
		}
	case "RESTORE":
		if len(args) < 2 {
			w.WriteError("ERR SNAPSHOT RESTORE requires an id")
			return
		}
		if err := s.engine.SnapshotRestore(args[1].Str); err != nil {
			w.WriteError(err.Error())
			return
		}
		w.WriteSimpleString("OK")
	case "DELETE":
		if len(args) < 2 {
			w.WriteError("ERR SNAPSHOT DELETE requires an id")
			return
		}
		if err := s.engine.SnapshotDelete(args[1].Str); err != nil {
			w.WriteError(err.Error())
			return
		}
		w.WriteSimpleString("OK")
	default:
		w.WriteError(fmt.Sprintf("ERR unknown SNAPSHOT subcommand '%s'", sub))
	}
}

// ========================
// Phase 6: CDC Commands
// ========================

func (s *Server) cmdCDC(w *protocol.Writer, args []protocol.Value) {
	if len(args) < 1 {
		w.WriteError("wrong number of arguments for 'CDC' command")
		return
	}
	sub := strings.ToUpper(args[0].Str)
	switch sub {
	case "LATEST":
		n := 50
		if len(args) >= 2 {
			var err error
			n, err = strconv.Atoi(args[1].Str)
			if err != nil || n <= 0 {
				w.WriteError("ERR invalid count")
				return
			}
		}
		events := s.engine.CDCLatest(n)
		w.WriteArrayHeader(len(events))
		for _, ev := range events {
			w.WriteArrayHeader(8)
			w.WriteBulkString([]byte("id"))
			w.WriteInteger(int64(ev.ID))
			w.WriteBulkString([]byte("op"))
			w.WriteBulkString([]byte(string(ev.Op)))
			w.WriteBulkString([]byte("key"))
			w.WriteBulkString([]byte(ev.Key))
		}
	case "SINCE":
		if len(args) < 2 {
			w.WriteError("ERR CDC SINCE requires an after-id")
			return
		}
		afterID, err := strconv.ParseUint(args[1].Str, 10, 64)
		if err != nil {
			w.WriteError("ERR invalid id")
			return
		}
		events := s.engine.CDCSince(afterID)
		w.WriteArrayHeader(len(events))
		for _, ev := range events {
			w.WriteArrayHeader(8)
			w.WriteBulkString([]byte("id"))
			w.WriteInteger(int64(ev.ID))
			w.WriteBulkString([]byte("op"))
			w.WriteBulkString([]byte(string(ev.Op)))
			w.WriteBulkString([]byte("key"))
			w.WriteBulkString([]byte(ev.Key))
		}
	case "STATS":
		stats := s.engine.CDCStats()
		w.WriteArrayHeader(8)
		w.WriteBulkString([]byte("total_events"))
		w.WriteInteger(int64(stats.TotalEvents))
		w.WriteBulkString([]byte("buffer_size"))
		w.WriteInteger(int64(stats.BufferSize))
		w.WriteBulkString([]byte("buffer_cap"))
		w.WriteInteger(int64(stats.BufferCap))
		w.WriteBulkString([]byte("subscribers"))
		w.WriteInteger(int64(stats.Subscribers))
	default:
		w.WriteError(fmt.Sprintf("ERR unknown CDC subcommand '%s'", sub))
	}
}

// ========================
// Phase 6: Built-in Benchmark
// ========================

func (s *Server) cmdBenchmark(w *protocol.Writer, args []protocol.Value) {
	n := 1000
	if len(args) >= 1 {
		var err error
		n, err = strconv.Atoi(args[0].Str)
		if err != nil || n <= 0 {
			w.WriteError("ERR invalid operation count")
			return
		}
	}
	result := s.engine.RunBenchmark(n)
	w.WriteArrayHeader(16)
	w.WriteBulkString([]byte("operations"))
	w.WriteInteger(int64(result.Operations))
	w.WriteBulkString([]byte("duration_ms"))
	w.WriteInteger(result.Duration / 1e6)
	w.WriteBulkString([]byte("ops_per_sec"))
	w.WriteBulkString([]byte(strconv.FormatFloat(result.OpsPerSec, 'f', 0, 64)))
	w.WriteBulkString([]byte("avg_latency_ns"))
	w.WriteInteger(result.AvgLatencyNs)
	w.WriteBulkString([]byte("set_ops_per_sec"))
	w.WriteBulkString([]byte(strconv.FormatFloat(result.SetOpsPerSec, 'f', 0, 64)))
	w.WriteBulkString([]byte("get_ops_per_sec"))
	w.WriteBulkString([]byte(strconv.FormatFloat(result.GetOpsPerSec, 'f', 0, 64)))
	w.WriteBulkString([]byte("p99_latency_ns"))
	w.WriteInteger(result.P99LatencyNs)
	w.WriteBulkString([]byte("concurrency"))
	w.WriteInteger(int64(result.Concurrency))
}
