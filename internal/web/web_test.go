package web

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/flashdb/flashdb/internal/engine"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func newTestWebServer(t *testing.T) *Server {
	t.Helper()

	walPath := filepath.Join(t.TempDir(), "test.wal")
	e, err := engine.New(walPath)
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = e.Close()
	})

	return New(":0", e)
}

func TestExecuteLegacyAndV1(t *testing.T) {
	s := newTestWebServer(t)
	handler := corsMiddleware(s.routes())

	body := []byte(`{"command":"PING"}`)

	for _, path := range []string{"/api/execute", "/api/v1/execute"} {
		req := httptest.NewRequest(http.MethodPost, path, bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		require.Equal(t, http.StatusOK, rr.Code)

		var resp CommandResponse
		require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &resp))
		assert.True(t, resp.Success)
		assert.Equal(t, "PONG", resp.Result)
	}
}

func TestHealthAndReadinessEndpoints(t *testing.T) {
	s := newTestWebServer(t)
	handler := corsMiddleware(s.routes())

	for _, path := range []string{"/healthz", "/readyz", "/api/v1/healthz", "/api/v1/readyz"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)
		assert.Equal(t, http.StatusOK, rr.Code)
		assert.Contains(t, rr.Body.String(), "status")
	}
}

func TestVersionedKeyRoute(t *testing.T) {
	s := newTestWebServer(t)
	handler := corsMiddleware(s.routes())

	setReq := httptest.NewRequest(http.MethodPost, "/api/v1/execute", bytes.NewReader([]byte(`{"command":"SET mykey value"}`)))
	setReq.Header.Set("Content-Type", "application/json")
	setResp := httptest.NewRecorder()
	handler.ServeHTTP(setResp, setReq)
	require.Equal(t, http.StatusOK, setResp.Code)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/key/mykey", nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	require.Equal(t, http.StatusOK, rr.Code)

	var info KeyInfo
	require.NoError(t, json.Unmarshal(rr.Body.Bytes(), &info))
	assert.Equal(t, "mykey", info.Key)
	assert.Equal(t, "value", info.Value)
}
