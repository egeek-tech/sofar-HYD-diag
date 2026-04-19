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
	web.SetupRoutes(r, b, h, defaults, time.Now(), "test-v1.0.0", logger)
	return r
}

func TestStatusEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	var resp web.StatusResponse
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if resp.ConnectionState != "dormant" {
		t.Errorf("expected connection_state 'dormant', got %q", resp.ConnectionState)
	}
	if resp.InverterAddr != "127.0.0.1:1" {
		t.Errorf("expected inverter_addr '127.0.0.1:1', got %q", resp.InverterAddr)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
}

func TestDefaultsEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/api/defaults", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp web.DefaultsConfig
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}
	if resp.Host != "10.5.99.29" {
		t.Errorf("expected host '10.5.99.29', got %q", resp.Host)
	}
	if resp.Port != 4192 {
		t.Errorf("expected port 4192, got %d", resp.Port)
	}
	if resp.SlaveID != 1 {
		t.Errorf("expected slave_id 1, got %d", resp.SlaveID)
	}
	if resp.PVChannels != 2 {
		t.Errorf("expected pv_channels 2, got %d", resp.PVChannels)
	}
}

func TestHealthzEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}

func TestReadyzEndpointDormant(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Broker starts in StateDormant -- readyz should return 503
	if w.Code != http.StatusServiceUnavailable {
		t.Fatalf("expected 503 for dormant broker, got %d", w.Code)
	}
}

func TestStatusInfoEndpoint(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/status", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	ct := w.Header().Get("Content-Type")
	if !strings.Contains(ct, "application/json") {
		t.Errorf("expected Content-Type application/json, got %q", ct)
	}

	var resp web.StatusInfo
	if err := json.NewDecoder(w.Body).Decode(&resp); err != nil {
		t.Fatalf("failed to decode JSON: %v", err)
	}

	if resp.Status != "ok" {
		t.Errorf("expected status 'ok', got %q", resp.Status)
	}
	if resp.Version != "test-v1.0.0" {
		t.Errorf("expected version 'test-v1.0.0', got %q", resp.Version)
	}
	if resp.Broker != "dormant" {
		t.Errorf("expected broker 'dormant', got %q", resp.Broker)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
	}
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
	web.SetupRoutes(r, b, h, defaults, time.Now(), "test-v1.0.0", logger)

	srv := httptest.NewServer(r)
	defer srv.Close()

	wsURL := "ws" + strings.TrimPrefix(srv.URL, "http") + "/ws"
	conn, resp, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("ws dial failed: %v", err)
	}
	defer conn.Close()

	if resp.StatusCode != 101 {
		t.Errorf("expected 101, got %d", resp.StatusCode)
	}

	// Should receive initial connection_state message
	conn.SetReadDeadline(time.Now().Add(2 * time.Second))
	_, msg, err := conn.ReadMessage()
	if err != nil {
		t.Fatalf("read initial message failed: %v", err)
	}
	if !strings.Contains(string(msg), "connection_state") {
		t.Errorf("expected connection_state message, got: %s", msg)
	}
}

func TestWSUpgradeWithoutHeaders(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/ws", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Without upgrade headers, the WS upgrader should reject with 400
	if w.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", w.Code)
	}
}

func TestStaticFileServing(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "<title>Sofar HYD Diagnostic Tool</title>") {
		t.Error("expected index.html to contain the title")
	}
}

func TestStaticCSS(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/style.css", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}

	body := w.Body.String()
	if !strings.Contains(body, "font-family") {
		t.Error("expected style.css to contain font-family")
	}
}

func TestStaticJS(t *testing.T) {
	r := newTestRouter()
	req := httptest.NewRequest(http.MethodGet, "/app.js", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200, got %d", w.Code)
	}
}
