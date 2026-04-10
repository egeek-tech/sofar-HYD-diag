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
	batInputs    = flag.Int("bat-inputs", 1, "Default battery inputs (1-2)")
	batTowers    = flag.Int("bat-towers", 2, "Default towers per input (1-4)")
	batPacks     = flag.Int("bat-packs", 10, "Default packs per tower (4-10)")
	logLevel     = flag.String("log-level", "info", "Log level: debug, info, warn, error")
)

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

func main() {
	flag.Parse()

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
	if *batInputs < 1 || *batInputs > 2 {
		fmt.Fprintf(os.Stderr, "error: bat-inputs must be 1-2, got %d\n", *batInputs)
		os.Exit(1)
	}
	if *batTowers < 1 || *batTowers > 4 {
		fmt.Fprintf(os.Stderr, "error: bat-towers must be 1-4, got %d\n", *batTowers)
		os.Exit(1)
	}
	if *batPacks < 4 || *batPacks > 10 {
		fmt.Fprintf(os.Stderr, "error: bat-packs must be 4-10, got %d\n", *batPacks)
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
	b := broker.New(logger.With("component", "broker"), inverterAddr, byte(*slaveID), useRTU)
	go b.Run(ctx)

	// Create WebSocket hub (D-03, D-29)
	wsHub := hub.NewHub(b, logger.With("component", "hub"), *pvChannels, *batInputs, *batTowers, *batPacks)
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
		BatInputs:  *batInputs,
		BatTowers:  *batTowers,
		BatPacks:   *batPacks,
	}
	web.SetupRoutes(r, b, wsHub, defaults, startTime, logger)

	// Create HTTP server
	srv := &http.Server{
		Addr:    *listenAddr,
		Handler: r,
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
