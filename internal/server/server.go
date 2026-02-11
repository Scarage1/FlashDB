// Package server implements the TCP server for FlashDB using RESP protocol.
package server

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/flashdb/flashdb/internal/protocol"
	"github.com/flashdb/flashdb/internal/store"
)

// Version is the FlashDB version string.
const Version = "1.0.0"

// Config holds server configuration.
type Config struct {
	Password   string
	MaxClients int
	Timeout    time.Duration
	LogLevel   string
}

// DefaultConfig returns default server configuration.
func DefaultConfig() Config {
	return Config{
		Password:   "",
		MaxClients: 10000,
		Timeout:    0,
		LogLevel:   "info",
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
}

// New creates a new Server with the specified address and engine.
func New(addr string, e *engine.Engine) *Server {
	return NewWithConfig(addr, e, DefaultConfig())
}

// NewWithConfig creates a new Server with the specified configuration.
func NewWithConfig(addr string, e *engine.Engine, cfg Config) *Server {
	return &Server{
		addr:      addr,
		engine:    e,
		config:    cfg,
		clients:   make(map[int64]*clientConn),
		startTime: time.Now(),
		pubsub:    NewPubSub(),
	}
}

// Start starts the server and listens for connections.
// It blocks until the context is cancelled.
func (s *Server) Start(ctx context.Context) error {
	listener, err := net.Listen("tcp", s.addr)
	if err != nil {
		return fmt.Errorf("server: failed to listen: %w", err)
	}

	s.mu.Lock()
	s.listener = listener
	s.mu.Unlock()

	log.Printf("FlashDB server listening on %s", s.addr)
	if s.config.Password != "" {
		log.Printf("Authentication enabled")
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
			log.Printf("server: failed to accept connection: %v", err)
			continue
		}

		// Check max clients
		s.mu.RLock()
		currentClients := len(s.clients)
		s.mu.RUnlock()

		if s.config.MaxClients > 0 && currentClients >= s.config.MaxClients {
			conn.Close()
			log.Printf("server: max clients reached, rejecting connection")
			continue
		}

		// Create client connection
		s.mu.Lock()
		s.nextConnID++
		connID := s.nextConnID
		client := &clientConn{
			id:             connID,
			conn:           conn,
			addr:           conn.RemoteAddr().String(),
			authenticated:  s.config.Password == "", // Auto-auth if no password
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

		// Extract command and args
		cmd := strings.ToUpper(val.Array[0].Str)
		args := val.Array[1:]

		// Update client stats
		client.lastCommand = time.Now()
		atomic.AddInt64(&client.cmdCount, 1)
		atomic.AddInt64(&s.totalCmds, 1)

		// Check authentication for non-AUTH commands
		if !client.authenticated && cmd != "AUTH" && cmd != "PING" && cmd != "QUIT" {
			writer.WriteError("NOAUTH Authentication required")
			continue
		}

		s.executeCommand(writer, client, cmd, args)
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

	for i := 0; i < len(args); i += 2 {
		key := args[i].Str
		value := []byte(args[i+1].Str)
		if err := s.engine.Set(key, value); err != nil {
			w.WriteError("internal error")
			return
		}
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

	// Only support '*' pattern (all keys)
	if pattern != "*" {
		// For simplicity, only support * pattern
		w.WriteStringArray([]string{})
		return
	}

	w.WriteStringArray(keys)
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

	if s.engine.Exists(args[0].Str) {
		w.WriteSimpleString("string")
	} else {
		w.WriteSimpleString("none")
	}
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

// AUTH command
func (s *Server) cmdAuth(w *protocol.Writer, client *clientConn, args []protocol.Value) {
	if len(args) != 1 {
		w.WriteError("wrong number of arguments for 'AUTH' command")
		return
	}

	if s.config.Password == "" {
		w.WriteError("Client sent AUTH, but no password is set")
		return
	}

	if args[0].Str == s.config.Password {
		client.authenticated = true
		w.WriteSimpleString("OK")
	} else {
		w.WriteError("WRONGPASS invalid username-password pair")
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

	// Get all keys and filter
	allKeys := s.engine.Keys()
	var matched []string

	// Convert pattern to regex
	regexPattern := "^" + strings.ReplaceAll(strings.ReplaceAll(pattern, "*", ".*"), "?", ".") + "$"
	re, err := regexp.Compile(regexPattern)
	if err != nil {
		re = regexp.MustCompile(".*") // Match all on invalid pattern
	}

	for _, key := range allKeys {
		if re.MatchString(key) {
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
	// Return first key (simple implementation)
	w.WriteBulkString([]byte(keys[0]))
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

	// Execute all queued commands
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

	// Check if any key exists
	for i := 0; i < len(args); i += 2 {
		if s.engine.Exists(args[i].Str) {
			w.WriteInteger(0)
			return
		}
	}

	// Set all keys
	for i := 0; i < len(args); i += 2 {
		key := args[i].Str
		value := []byte(args[i+1].Str)
		if err := s.engine.Set(key, value); err != nil {
			w.WriteError("internal error")
			return
		}
	}

	w.WriteInteger(1)
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

	var current float64
	value, exists := s.engine.Get(key)
	if exists {
		current, err = strconv.ParseFloat(string(value), 64)
		if err != nil {
			w.WriteError("value is not a valid float")
			return
		}
	}

	newValue := current + increment
	newStr := strconv.FormatFloat(newValue, 'f', -1, 64)

	if err := s.engine.Set(key, []byte(newStr)); err != nil {
		w.WriteError("internal error")
		return
	}

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

	added := s.engine.ZAdd(key, members...)
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

	removed := s.engine.ZRem(key, members...)
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

	newScore := s.engine.ZIncrBy(key, member, increment)
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

	removed := s.engine.ZRemRangeByRank(key, start, stop)
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

	removed := s.engine.ZRemRangeByScore(key, min, max)
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

	members := s.engine.ZPopMin(key, count)
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

	members := s.engine.ZPopMax(key, count)
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
