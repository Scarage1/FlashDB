package server

import (
	"bufio"
	"context"
	"fmt"
	"net"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/flashdb/flashdb/internal/protocol"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupTestServer(t *testing.T) (*Server, string) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := engine.New(walPath)
	require.NoError(t, err)

	addr := "127.0.0.1:0"
	s := New(addr, e)

	return s, tmpDir
}

func startTestServer(t *testing.T, s *Server) string {
	listener, err := net.Listen("tcp", s.addr)
	require.NoError(t, err)

	actualAddr := listener.Addr().String()
	ctx, cancel := context.WithCancel(context.Background())
	var connID int64

	go func() {
		for {
			conn, err := listener.Accept()
			if err != nil {
				return
			}
			connID++
			client := &clientConn{
				id:             connID,
				conn:           conn,
				addr:           conn.RemoteAddr().String(),
				authenticated:  true, // No password in tests
				createdAt:      time.Now(),
				lastCommand:    time.Now(),
				subscriptions:  make(map[string]bool),
				psubscriptions: make(map[string]bool),
			}
			go s.handleConnection(ctx, client)
		}
	}()

	t.Cleanup(func() {
		cancel()
		listener.Close()
		s.engine.Close()
	})

	return actualAddr
}

func sendCommand(t *testing.T, addr string, args ...string) string {
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(2 * time.Second))

	// Convert args to [][]byte for WriteArray
	byteArgs := make([][]byte, len(args))
	for i, arg := range args {
		byteArgs[i] = []byte(arg)
	}

	writer := protocol.NewWriter(conn)
	err = writer.WriteArray(byteArgs)
	require.NoError(t, err)

	reader := protocol.NewReader(conn)
	resp, err := reader.ReadValue()
	require.NoError(t, err)

	switch resp.Type {
	case protocol.TypeSimpleString:
		return resp.Str
	case protocol.TypeBulkString:
		if resp.Null {
			return "(nil)"
		}
		return resp.Str
	case protocol.TypeInteger:
		return fmt.Sprintf("%d", resp.Num)
	case protocol.TypeArray:
		var parts []string
		for _, item := range resp.Array {
			if item.Null {
				parts = append(parts, "(nil)")
			} else {
				parts = append(parts, item.Str)
			}
		}
		return strings.Join(parts, ",")
	case protocol.TypeError:
		return "ERR: " + resp.Str
	default:
		return ""
	}
}

func TestServer_PING(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "PING")
	assert.Equal(t, "PONG", resp)
}

func TestServer_SetAndGet(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "SET", "key1", "value1")
	assert.Equal(t, "OK", resp)

	resp = sendCommand(t, addr, "GET", "key1")
	assert.Equal(t, "value1", resp)
}

func TestServer_DEL(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")
	sendCommand(t, addr, "SET", "key2", "value2")

	resp := sendCommand(t, addr, "DEL", "key1", "key2", "key3")
	assert.Equal(t, "2", resp)
}

func TestServer_EXISTS(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")

	resp := sendCommand(t, addr, "EXISTS", "key1", "key2")
	assert.Equal(t, "1", resp)
}

func TestServer_INCR_DECR(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "INCR", "counter")
	assert.Equal(t, "1", resp)

	resp = sendCommand(t, addr, "INCRBY", "counter", "10")
	assert.Equal(t, "11", resp)

	resp = sendCommand(t, addr, "DECR", "counter")
	assert.Equal(t, "10", resp)

	resp = sendCommand(t, addr, "DECRBY", "counter", "5")
	assert.Equal(t, "5", resp)
}

func TestServer_SETEX(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "SETEX", "key1", "1", "value1")
	assert.Equal(t, "OK", resp)

	resp = sendCommand(t, addr, "GET", "key1")
	assert.Equal(t, "value1", resp)
}

func TestServer_SETNX(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "SETNX", "key1", "value1")
	assert.Equal(t, "1", resp)

	resp = sendCommand(t, addr, "SETNX", "key1", "value2")
	assert.Equal(t, "0", resp)
}

func TestServer_APPEND(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "Hello")
	resp := sendCommand(t, addr, "APPEND", "key1", " World")
	assert.Equal(t, "11", resp)

	resp = sendCommand(t, addr, "GET", "key1")
	assert.Equal(t, "Hello World", resp)
}

func TestServer_STRLEN(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "Hello")
	resp := sendCommand(t, addr, "STRLEN", "key1")
	assert.Equal(t, "5", resp)
}

func TestServer_DBSIZE(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")
	sendCommand(t, addr, "SET", "key2", "value2")

	resp := sendCommand(t, addr, "DBSIZE")
	assert.Equal(t, "2", resp)
}

func TestServer_FLUSHDB(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")
	sendCommand(t, addr, "SET", "key2", "value2")

	resp := sendCommand(t, addr, "FLUSHDB")
	assert.Equal(t, "OK", resp)

	resp = sendCommand(t, addr, "DBSIZE")
	assert.Equal(t, "0", resp)
}

func TestServer_TYPE(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")

	resp := sendCommand(t, addr, "TYPE", "key1")
	assert.Equal(t, "string", resp)

	resp = sendCommand(t, addr, "TYPE", "nonexistent")
	assert.Equal(t, "none", resp)
}

func TestServer_UnknownCommand(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "UNKNOWNCOMMAND")
	assert.Contains(t, resp, "ERR")
}

func TestServer_RENAME(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "oldkey", "value1")
	resp := sendCommand(t, addr, "RENAME", "oldkey", "newkey")
	assert.Equal(t, "OK", resp)

	resp = sendCommand(t, addr, "GET", "newkey")
	assert.Equal(t, "value1", resp)

	resp = sendCommand(t, addr, "EXISTS", "oldkey")
	assert.Equal(t, "0", resp)
}

func TestServer_SELECT(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "SELECT", "0")
	assert.Equal(t, "OK", resp)
}

func TestServer_CONFIG(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "CONFIG", "GET", "maxclients")
	assert.Contains(t, resp, "maxclients")
}

func TestServer_MEMORY(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "value1")
	resp := sendCommand(t, addr, "MEMORY", "USAGE", "key1")
	assert.NotEmpty(t, resp)
}

func TestServer_TIME(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "TIME")
	assert.NotEmpty(t, resp)
}

func TestServer_INFO(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	resp := sendCommand(t, addr, "INFO")
	assert.Contains(t, resp, "flashdb_version")
}

func TestServer_Transaction(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	// For transactions, we need a persistent connection
	conn, err := net.DialTimeout("tcp", addr, 2*time.Second)
	require.NoError(t, err)
	defer conn.Close()

	conn.SetDeadline(time.Now().Add(5 * time.Second))
	writer := protocol.NewWriter(conn)
	reader := protocol.NewReader(conn)

	// Helper to send and receive
	send := func(args ...string) string {
		byteArgs := make([][]byte, len(args))
		for i, arg := range args {
			byteArgs[i] = []byte(arg)
		}
		err := writer.WriteArray(byteArgs)
		require.NoError(t, err)
		resp, err := reader.ReadValue()
		require.NoError(t, err)
		switch resp.Type {
		case protocol.TypeSimpleString:
			return resp.Str
		case protocol.TypeBulkString:
			if resp.Null {
				return "(nil)"
			}
			return resp.Str
		case protocol.TypeInteger:
			return fmt.Sprintf("%d", resp.Num)
		case protocol.TypeError:
			return "ERR: " + resp.Str
		default:
			return fmt.Sprintf("%v", resp)
		}
	}

	// Test MULTI/EXEC
	resp := send("MULTI")
	assert.Equal(t, "OK", resp)

	resp = send("SET", "tx1", "value1")
	assert.Equal(t, "QUEUED", resp)

	resp = send("SET", "tx2", "value2")
	assert.Equal(t, "QUEUED", resp)

	resp = send("EXEC")
	assert.NotEmpty(t, resp)

	// Verify values were set
	resp = send("GET", "tx1")
	assert.Equal(t, "value1", resp)

	resp = send("GET", "tx2")
	assert.Equal(t, "value2", resp)

	// Test DISCARD
	resp = send("MULTI")
	assert.Equal(t, "OK", resp)

	send("SET", "discarded", "value")
	resp = send("DISCARD")
	assert.Equal(t, "OK", resp)

	resp = send("GET", "discarded")
	assert.Equal(t, "(nil)", resp)
}

func TestServer_GETSET(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	// GETSET on non-existent key
	resp := sendCommand(t, addr, "GETSET", "newkey", "newvalue")
	assert.Equal(t, "(nil)", resp)

	// GETSET on existing key
	resp = sendCommand(t, addr, "GETSET", "newkey", "updated")
	assert.Equal(t, "newvalue", resp)

	// Verify new value
	resp = sendCommand(t, addr, "GET", "newkey")
	assert.Equal(t, "updated", resp)
}

func TestServer_GETDEL(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "todelete", "myvalue")

	resp := sendCommand(t, addr, "GETDEL", "todelete")
	assert.Equal(t, "myvalue", resp)

	// Should be deleted now
	resp = sendCommand(t, addr, "GET", "todelete")
	assert.Equal(t, "(nil)", resp)
}

func TestServer_GETRANGE(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "mykey", "Hello World")

	resp := sendCommand(t, addr, "GETRANGE", "mykey", "0", "4")
	assert.Equal(t, "Hello", resp)

	// Negative indices
	resp = sendCommand(t, addr, "GETRANGE", "mykey", "-5", "-1")
	assert.Equal(t, "World", resp)
}

func TestServer_SETRANGE(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "key1", "Hello World")

	resp := sendCommand(t, addr, "SETRANGE", "key1", "6", "Redis")
	assert.Equal(t, "11", resp) // Length

	resp = sendCommand(t, addr, "GET", "key1")
	assert.Equal(t, "Hello Redis", resp)
}

func TestServer_MSETNX(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	// All keys new
	resp := sendCommand(t, addr, "MSETNX", "k1", "v1", "k2", "v2")
	assert.Equal(t, "1", resp)

	// One key exists
	resp = sendCommand(t, addr, "MSETNX", "k1", "new", "k3", "v3")
	assert.Equal(t, "0", resp)

	// k3 should NOT exist
	resp = sendCommand(t, addr, "GET", "k3")
	assert.Equal(t, "(nil)", resp)
}

func TestServer_INCRBYFLOAT(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "floatkey", "10.5")

	resp := sendCommand(t, addr, "INCRBYFLOAT", "floatkey", "0.1")
	assert.Equal(t, "10.6", resp)
}

func TestServer_TOUCH(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "k1", "v1")
	sendCommand(t, addr, "SET", "k2", "v2")

	resp := sendCommand(t, addr, "TOUCH", "k1", "k2", "nonexistent")
	assert.Equal(t, "2", resp)
}

func TestServer_OBJECT(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "objkey", "value")

	resp := sendCommand(t, addr, "OBJECT", "ENCODING", "objkey")
	assert.Equal(t, "raw", resp)

	resp = sendCommand(t, addr, "OBJECT", "REFCOUNT", "objkey")
	assert.Equal(t, "1", resp)
}

func TestServer_COPY(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "source", "value")

	resp := sendCommand(t, addr, "COPY", "source", "dest")
	assert.Equal(t, "1", resp)

	resp = sendCommand(t, addr, "GET", "dest")
	assert.Equal(t, "value", resp)

	// Source still exists
	resp = sendCommand(t, addr, "GET", "source")
	assert.Equal(t, "value", resp)
}

func TestServer_UNLINK(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	sendCommand(t, addr, "SET", "toremove", "value")

	resp := sendCommand(t, addr, "UNLINK", "toremove")
	assert.Equal(t, "1", resp)

	resp = sendCommand(t, addr, "EXISTS", "toremove")
	assert.Equal(t, "0", resp)
}

func TestServer_PubSub(t *testing.T) {
	s, _ := setupTestServer(t)
	addr := startTestServer(t, s)

	// PUBSUB CHANNELS should return empty initially
	resp := sendCommand(t, addr, "PUBSUB", "CHANNELS")
	assert.Equal(t, "", resp) // Empty array returns empty string

	// PUBSUB NUMPAT
	resp = sendCommand(t, addr, "PUBSUB", "NUMPAT")
	assert.Equal(t, "0", resp)
}

func TestWALFilePath(t *testing.T) {
	tmpDir := t.TempDir()
	walPath := filepath.Join(tmpDir, "test.wal")
	e, err := engine.New(walPath)
	require.NoError(t, err)
	defer e.Close()

	_, err = os.Stat(walPath)
	assert.NoError(t, err)
}

// Suppress unused import warning
var _ = bufio.Reader{}
