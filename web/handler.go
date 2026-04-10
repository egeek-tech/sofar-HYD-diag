package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"sofar-hyd-diag/internal/broker"
)

//go:embed static/*
var staticFiles embed.FS

// StatusResponse represents the JSON response for /api/status.
type StatusResponse struct {
	Uptime          string `json:"uptime"`
	ConnectionState string `json:"connection_state"`
	InverterAddr    string `json:"inverter_addr"`
}

// SetupRoutes configures the chi router with API endpoints and embedded static file serving.
func SetupRoutes(r chi.Router, b *broker.Broker, startTime time.Time) {
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
