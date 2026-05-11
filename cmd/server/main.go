// Command server is the Sofar HYD diagnostic web binary: it wires CLI
// configuration to the broker, hub, and HTTP/WebSocket routes.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
	"sofar-hyd-diag/internal/broker"
	"sofar-hyd-diag/internal/hub"
	"sofar-hyd-diag/web"
)

var (
	listenAddr   = flag.String("listen", ":8080", "HTTP listen address (host:port)")
	inverterHost = flag.String("inverter-host", "10.5.99.29", "Inverter IP address (pre-populates browser UI)")
	inverterPort = flag.Int("inverter-port", 4192, "Inverter TCP port")
	slaveID      = flag.Int("slave", 1, "Modbus slave ID (1-247)")
	modbusMode   = flag.String("modbus-mode", "tcp", "Modbus protocol mode: tcp or rtu")
	pvChannels   = flag.Int("pv-channels", 2, "Default number of PV channels (2-16, pre-populates browser dropdown)")
	logLevel     = flag.String("log-level", "info", "Log level: debug, info, warn, error")
)

// version is set at build time via -ldflags "-X main.version=..."
var version = "dev"

// setupLogger creates an slog.Logger with the specified log level.
// Supported levels: debug, info, warn, error (case-insensitive).
func setupLogger(levelName string) *slog.Logger {
	var level slog.LevelVar
	switch strings.ToLower(levelName) {
	case "debug":
		level.Set(slog.LevelDebug)
	case "warn":
		level.Set(slog.LevelWarn)
	case "error":
		level.Set(slog.LevelError)
	default:
		level.Set(slog.LevelInfo)
	}
	handler := slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: &level})
	return slog.New(handler)
}

// applyEnvOverrides sets flag values from environment variables
// for flags that were not explicitly set on the command line.
// Precedence: explicit flag > env var > compiled default (D-05).
func applyEnvOverrides() {
	explicitly := make(map[string]bool)
	flag.Visit(func(f *flag.Flag) {
		explicitly[f.Name] = true
	})

	envMap := map[string]string{
		"listen":        "LISTEN_ADDR",
		"inverter-host": "INVERTER_HOST",
		"inverter-port": "INVERTER_PORT",
		"slave":         "SLAVE_ID",
		"modbus-mode":   "MODBUS_MODE",
		"pv-channels":   "PV_CHANNELS",
		"log-level":     "LOG_LEVEL",
	}

	for flagName, envName := range envMap {
		if explicitly[flagName] {
			continue
		}
		if val, ok := os.LookupEnv(envName); ok {
			_ = flag.Set(flagName, val)
		}
	}
}

func main() {
	flag.Parse()
	applyEnvOverrides()

	// Validate inputs
	if *slaveID < 1 || *slaveID > 247 {
		fmt.Fprintf(os.Stderr, "error: slave ID must be 1-247, got %d\n", *slaveID)
		os.Exit(1)
	}
	if *modbusMode != "tcp" && *modbusMode != "rtu" {
		fmt.Fprintf(os.Stderr, "error: modbus-mode must be 'tcp' or 'rtu', got %q\n", *modbusMode)
		os.Exit(1)
	}
	if *pvChannels < 2 || *pvChannels > 16 {
		fmt.Fprintf(os.Stderr, "error: pv-channels must be 2-16, got %d\n", *pvChannels)
		os.Exit(1)
	}

	// Setup structured logger (INFRA-02)
	logger := setupLogger(*logLevel)

	// Setup signal context for graceful shutdown (D-27)
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// Create and start Modbus broker (D-13, D-14)
	inverterAddr := fmt.Sprintf("%s:%d", *inverterHost, *inverterPort)
	useRTU := *modbusMode == "rtu"
	b := broker.New(logger.With("component", "broker"), inverterAddr, byte(*slaveID), useRTU) //nolint:gosec // G115: slaveID validated to 1-247 above
	go b.Run(ctx)

	// Create WebSocket hub (D-03, D-29)
	wsHub := hub.NewHub(b, logger.With("component", "hub"), *pvChannels)
	go wsHub.Run(ctx)

	// Create chi router with middleware
	r := chi.NewRouter()
	r.Use(middleware.Recoverer)
	r.Use(middleware.RealIP)
	r.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			logger.Debug("http request", "method", req.Method, "path", req.URL.Path)
			next.ServeHTTP(w, req)
		})
	})

	// Setup web routes
	startTime := time.Now()
	defaults := web.DefaultsConfig{
		Host:       *inverterHost,
		Port:       *inverterPort,
		SlaveID:    *slaveID,
		PVChannels: *pvChannels,
	}
	web.SetupRoutes(r, b, wsHub, defaults, startTime, version, logger)

	// Create HTTP server. ReadHeaderTimeout mitigates Slowloris attacks (gosec G112).
	srv := &http.Server{
		Addr:              *listenAddr,
		Handler:           r,
		ReadHeaderTimeout: 10 * time.Second,
	}

	// Start HTTP server in goroutine
	serverErr := make(chan error, 1)
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			serverErr <- err
		}
	}()

	// Print startup message to stdout (D-28: no auto-open browser)
	logger.Info("server started", "addr", *listenAddr, "inverter", inverterAddr, "mode", *modbusMode)
	fmt.Printf("Sofar HYD Diagnostic Tool listening on http://localhost%s\n", *listenAddr)

	// Wait for shutdown signal or server error
	select {
	case <-ctx.Done():
	case err := <-serverErr:
		logger.Error("http server error", "error", err)
	}
	logger.Info("shutting down...")

	// Graceful shutdown with timeout (D-27)
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("http shutdown error", "error", err)
	}
	b.Close()
	logger.Info("shutdown complete")
}
