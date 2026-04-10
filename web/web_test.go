package web_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/go-chi/chi/v5"
	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/web"
)

// newTestRouter creates a chi router with web routes wired to a disconnected broker.
func newTestRouter() *chi.Mux {
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	b := broker.New(logger, "127.0.0.1:1", 1, false)
	r := chi.NewRouter()
	web.SetupRoutes(r, b, time.Now())
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

	if resp.ConnectionState != "disconnected" {
		t.Errorf("expected connection_state 'disconnected', got %q", resp.ConnectionState)
	}
	if resp.InverterAddr != "127.0.0.1:1" {
		t.Errorf("expected inverter_addr '127.0.0.1:1', got %q", resp.InverterAddr)
	}
	if resp.Uptime == "" {
		t.Error("expected non-empty uptime")
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
