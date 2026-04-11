package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/gorilla/websocket"
	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
)

//go:embed static/*
var staticFiles embed.FS

// StatusResponse represents the JSON response for /api/status.
type StatusResponse struct {
	Uptime          string `json:"uptime"`
	ConnectionState string `json:"connection_state"`
	InverterAddr    string `json:"inverter_addr"`
}

// DefaultsConfig holds CLI default values for the /api/defaults endpoint (D-14).
type DefaultsConfig struct {
	Host       string `json:"host"`
	Port       int    `json:"port"`
	SlaveID    int    `json:"slave_id"`
	PVChannels int    `json:"pv_channels"`
}

// upgrader configures WebSocket upgrade. CheckOrigin returns true because this is
// a local network diagnostic tool with no public internet exposure (T-02-10).
var upgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

// SetupRoutes configures the chi router with API endpoints, WebSocket handler, and embedded static file serving.
func SetupRoutes(r chi.Router, b *broker.Broker, h *hub.Hub, defaults DefaultsConfig, startTime time.Time, logger *slog.Logger) {
	// API routes
	r.Get("/api/status", func(w http.ResponseWriter, r *http.Request) {
		status := StatusResponse{
			Uptime:          time.Since(startTime).Round(time.Second).String(),
			ConnectionState: b.CurrentState().String(),
			InverterAddr:    b.Address(),
		}
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(status); err != nil {
			// Connection likely dropped; log for diagnostics but nothing to send back
			_ = err
		}
	})

	// GET /api/defaults returns CLI flag defaults as JSON (D-14).
	// Browser uses this to pre-populate the connection form.
	r.Get("/api/defaults", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(defaults); err != nil {
			_ = err
		}
	})

	// GET /ws upgrades HTTP to WebSocket and registers the client with the hub (D-06).
	r.Get("/ws", func(w http.ResponseWriter, r *http.Request) {
		conn, err := upgrader.Upgrade(w, r, nil)
		if err != nil {
			logger.Error("websocket upgrade failed", "error", err)
			return
		}
		client := hub.NewClient(h, conn, logger.With("component", "ws-client"))
		h.Register(client)
		go client.WritePump()
		go client.ReadPump()
	})

	// Serve embedded static files at root /
	staticFS, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(fmt.Sprintf("embedded static FS not found: %v", err))
	}
	fileServer(r, "/", http.FS(staticFS))
}

// fileServer serves static files from an embedded filesystem using chi routing.
func fileServer(r chi.Router, path string, root http.FileSystem) {
	if path != "/" && path[len(path)-1] != '/' {
		r.Get(path, http.RedirectHandler(path+"/", http.StatusMovedPermanently).ServeHTTP)
		path += "/"
	}
	path += "*"

	r.Get(path, func(w http.ResponseWriter, r *http.Request) {
		rctx := chi.RouteContext(r.Context())
		pathPrefix := strings.TrimSuffix(rctx.RoutePattern(), "/*")
		fs := http.StripPrefix(pathPrefix, http.FileServer(root))
		fs.ServeHTTP(w, r)
	})
}
