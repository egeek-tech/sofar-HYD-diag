package web_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
	"sofar-hyd-diag/web"
)

// newTestRouter creates a chi router with web routes wired to a disconnected broker and hub.
func newTestRouter() *chi.Mux {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := broker.New(logger, "127.0.0.1:1", 1, false)
	h := hub.NewHub(b, logger, 2)
	ctx, cancel := context.WithCancel(context.Background())
	go h.Run(ctx)
	_ = cancel // cleanup happens when test ends (short-lived)

	defaults := web.DefaultsConfig{
		Host:       "10.5.99.29",
		Port:       4192,
		SlaveID:    1,
		PVChannels: 2,
	}
	r := chi.NewRouter()
	web.SetupRoutes(r, b, h, defaults, time.Now(), logger)
	return r
}

func TestStatusEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200")

	var resp web.StatusResponse
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode JSON")

	assert.Equal(t, "dormant", resp.ConnectionState, "connection_state")
	assert.Equal(t, "127.0.0.1:1", resp.InverterAddr, "inverter_addr")
	assert.NotEmpty(t, resp.Uptime, "expected non-empty uptime")
}

func TestDefaultsEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/defaults", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200")

	ct := w.Header().Get("Content-Type")
	assert.Contains(t, ct, "application/json", "Content-Type")

	var resp web.DefaultsConfig
	require.NoError(t, json.NewDecoder(w.Body).Decode(&resp), "failed to decode JSON")
	assert.Equal(t, "10.5.99.29", resp.Host, "host")
	assert.Equal(t, 4192, resp.Port, "port")
	assert.Equal(t, 1, resp.SlaveID, "slave_id")
	assert.Equal(t, 2, resp.PVChannels, "pv_channels")
}

func TestWSUpgrade(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := broker.New(logger, "127.0.0.1:1", 1, false)
	h := hub.NewHub(b, logger, 2)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go h.Run(ctx)

	defaults := web.DefaultsConfig{Host: "10.5.99.29", Port: 4192, SlaveID: 1, PVChannels: 2}
	r := chi.NewRouter()
	web.SetupRoutes(r, b, h, defaults, time.Now(), logger)

	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	require.NoError(t, err, "ws dial failed")
	defer conn.Close()

	assert.Equal(t, 101, resp.StatusCode, "expected 101 Switching Protocols")

	// Should receive initial connection_state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	require.NoError(t, err, "read initial message failed")
	assert.Contains(t, string(msg), "connection_state", "expected connection_state message")
}

func TestWSUpgradeWithoutHeaders(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Without upgrade headers, the WS upgrader should reject with 400
	assert.Equal(t, http.StatusBadRequest, w.Code, "expected 400")
}

func TestStaticFileServing(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200")

	body := w.Body.String()
	assert.Contains(t, body, "<title>Sofar HYD Diagnostic Tool</title>", "expected index.html title")
}

func TestStaticCSS(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200")

	body := w.Body.String()
	assert.Contains(t, body, "font-family", "expected style.css to contain font-family")
}

func TestStaticJS(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.Equal(t, http.StatusOK, w.Code, "expected 200")
}
