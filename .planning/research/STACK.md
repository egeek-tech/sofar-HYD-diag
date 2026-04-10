# Technology Stack

**Project:** Sofar HYD Diagnostic Web Tool
**Researched:** 2026-04-10

## Existing Codebase Analysis

The current `main.go` (707 lines) is pure Go with zero external dependencies:
- Uses only standard library: `net`, `flag`, `fmt`, `log`, `encoding/binary`, `strings`, `sync/atomic`, `time`
- Raw TCP socket Modbus communication (both TCP and RTU modes)
- Connection retry logic with reconnect
- Function 0x03 (read holding registers) and 0x10 (write multiple registers)
- Already uses Go 1.26.1

**Key principle: Maintain zero-or-minimal dependency philosophy.** The existing code works precisely because it has no dependency chain. The web layer should follow the same ethos.

## Recommended Stack

### Go Version
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| Go | 1.26.x | Runtime | Already in go.mod (1.26.1). Go 1.22+ net/http routing eliminates need for third-party routers. |

**Confidence: HIGH** - Verified via go.mod and pkg.go.dev.

### HTTP Server / Router: `net/http` (standard library)
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| net/http + ServeMux | stdlib (Go 1.22+) | REST API, static file serving | Go 1.22 added method matching (`GET /api/registers`) and path parameters (`/api/battery/{pack}`). No third-party router needed for this project's ~10 endpoints. |

**Confidence: HIGH** - Verified Go 1.22+ routing features via pkg.go.dev. The enhanced ServeMux supports:
```go
mux.HandleFunc("GET /api/system", handleSystem)
mux.HandleFunc("GET /api/battery/{input}/{tower}/{pack}", handlePack)
mux.HandleFunc("POST /api/connect", handleConnect)
```

**Why not chi:** Chi (v5.2.5, Feb 2026) is excellent but adds a dependency for features this project does not need. Chi's value is route grouping, middleware composition, and sub-router mounting -- overkill for a single-purpose diagnostic tool with <15 routes. The standard library covers method dispatch, path parameters, and middleware chaining (via `http.Handler` wrapping).

**Why not echo/gin/fiber:** Heavy frameworks designed for large APIs. They pull in dozens of transitive dependencies, add complexity, and fight the single-binary-minimal-deps philosophy. Echo alone brings `golang.org/x/crypto`, `golang.org/x/net`, and more.

### WebSocket: `github.com/coder/websocket`
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| github.com/coder/websocket | v1.8.14 | Real-time push of register data | Zero dependencies, context.Context-native, zero-alloc reads/writes, works directly with net/http handlers. |

**Confidence: HIGH** - Verified v1.8.14 on pkg.go.dev (Sep 2025). Maintained by Coder (nhooyr transferred in 2024).

This library (formerly `nhooyr.io/websocket`) is the clear choice:
- **Zero external dependencies** -- aligns with project philosophy
- **net/http native** -- `websocket.Accept(w, r, nil)` works directly in http.HandlerFunc
- **Context-first** -- all operations accept context for timeout/cancellation, critical for Modbus timing
- **JSON helpers** -- `wsjson.Write(ctx, conn, data)` for sending register updates
- **Concurrent writes** -- safe to push updates from Modbus reader goroutine

**Why not gorilla/websocket:** gorilla/websocket (v1.5.3, Jun 2024) works but has not had a release in over a year. The gorilla project went through an archival scare in 2022-2023 and was revived by community maintainers, but activity remains sporadic. coder/websocket is actively maintained, has better ergonomics (context support, zero-alloc), and zero dependencies. gorilla/websocket requires manual ping/pong management and lacks context integration.

### Static File Embedding: `embed` (standard library)
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| embed | stdlib (Go 1.16+) | Bundle HTML/JS/CSS into binary | Single binary deployment. embed.FS implements io/fs.FS, works directly with http.FileServer. |

**Confidence: HIGH** - Standard library since Go 1.16, verified via pkg.go.dev.

Pattern:
```go
//go:embed static/*
var staticFiles embed.FS

// In main:
mux.Handle("GET /", http.FileServerFS(staticFiles))
```

### Structured Logging: `log/slog` (standard library)
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| log/slog | stdlib (Go 1.21+) | Structured backend logging | Standard library, zero deps, JSON output, dynamic level control, LogValuer for sensitive data. PROJECT.md explicitly requires structured logging. |

**Confidence: HIGH** - Standard library since Go 1.21, verified via pkg.go.dev.

Key usage for this project:
```go
// JSON handler for machine-readable logs
logger := slog.New(slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{
    Level: programLevel,
}))

// Modbus operation logging with context
logger.Info("register read",
    slog.String("register", "0x0604"),
    slog.Int("count", 1),
    slog.Duration("elapsed", elapsed),
)

// Dynamic level: expose via API for debugging
programLevel.Set(slog.LevelDebug)
```

**Why not zerolog/zap:** Both are faster than slog in micro-benchmarks, but:
1. This tool logs Modbus operations at ~2 reads/second. Performance is irrelevant at this scale.
2. zerolog and zap each add dependencies. zerolog pulls in `github.com/mattn/go-colorable` and friends.
3. slog is the Go standard. Every Go developer knows it. No docs to read.
4. slog's `LevelVar` for runtime level switching is perfect for diagnostic tools.

### Frontend: Vanilla HTML/JS/CSS
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| HTML5 + Vanilla JS + CSS | - | Diagnostic UI | PROJECT.md specifies vanilla. Desktop-only. No build step, no node_modules, no bundler. Embeds directly. |

**Confidence: HIGH** - PROJECT.md constraint, and correct for this use case.

This is a diagnostic tool with a fixed layout, not a dynamic SPA. The UI needs:
- Collapsible sections (HTML `<details>` element)
- Table rendering for register values
- WebSocket connection for live updates
- Green/red flash on update success/failure (CSS transitions)
- Connection config form

All achievable with vanilla JS. A framework (React, Vue, Svelte) would add a build step, require Node.js tooling, and complicate the embed workflow for zero benefit.

**Specific vanilla patterns to use:**
- `<template>` elements for repeating structures (battery pack rows)
- `EventSource` or `WebSocket` API (native browser)
- `fetch()` for REST calls
- CSS custom properties for theming (green/red feedback)
- `<details><summary>` for collapsible sections

### Configuration: `flag` + JSON config (standard library)
| Technology | Version | Purpose | Why |
|------------|---------|---------|-----|
| flag | stdlib | CLI arguments | Already used in main.go for host/port/slave. Extend for web port, log level. |
| encoding/json | stdlib | Optional config file | For saving/loading connection presets. No external config library needed. |

**Confidence: HIGH** - Already in use. Keep it.

## Complete Dependency List

```
go.mod dependencies (total: 1 external):
  github.com/coder/websocket v1.8.14   (zero transitive deps)
```

That is it. One external dependency with zero transitive dependencies. The entire web application adds exactly one line to go.mod.

## Alternatives Considered

| Category | Recommended | Alternative | Why Not |
|----------|-------------|-------------|---------|
| Router | net/http ServeMux | chi v5 | <15 routes, Go 1.22+ routing sufficient, avoids dependency |
| Router | net/http ServeMux | echo v4 | Heavy framework, many transitive deps, overkill |
| Router | net/http ServeMux | gin | Heavy framework, many transitive deps, overkill |
| WebSocket | coder/websocket | gorilla/websocket | Sporadic maintenance, no context support, manual ping/pong |
| WebSocket | coder/websocket | net/http (no WS) | SSE is one-directional; need bidirectional for connect/disconnect commands |
| Logging | log/slog | zerolog | External dep, irrelevant perf advantage at 2 ops/sec |
| Logging | log/slog | zap | External dep, complex API, irrelevant perf advantage |
| Frontend | Vanilla JS | htmx | Adds complexity for server-rendered approach that doesn't fit real-time WS updates |
| Frontend | Vanilla JS | Alpine.js | Small but still a dependency; vanilla JS handles this scope fine |
| Frontend | Vanilla JS | React/Vue/Svelte | Build step, node_modules, bundler -- massively overengineered for this |
| Config | flag + JSON | viper | Massive dependency tree for reading one config file |

## Build Pattern

Single binary with embedded frontend:

```go
package main

import (
    "embed"
    "log/slog"
    "net/http"
    "os"
)

//go:embed static/*
var staticFiles embed.FS

func main() {
    // Parse flags (host, port, slave, web-port, log-level)
    // Setup slog
    // Setup Modbus connection manager
    // Setup HTTP routes
    // Setup WebSocket hub
    // Serve
}
```

Build command:
```bash
go build -o sofar-diag .
```

Cross-compile for different targets:
```bash
GOOS=linux GOARCH=amd64 go build -o sofar-diag-linux .
GOOS=windows GOARCH=amd64 go build -o sofar-diag.exe .
GOOS=darwin GOARCH=arm64 go build -o sofar-diag-mac .
```

The result is a single binary (~10-15MB estimated) containing the Go runtime, Modbus communication layer, HTTP server, WebSocket support, and all static frontend files.

## Installation

```bash
# Add the single external dependency
go get github.com/coder/websocket@v1.8.14
```

No other installation steps. No npm, no node, no bundler, no Docker.

## Sources

- Go 1.22+ enhanced routing: https://pkg.go.dev/net/http (verified, Go 1.26.2 docs)
- coder/websocket v1.8.14: https://pkg.go.dev/github.com/coder/websocket (verified)
- gorilla/websocket v1.5.3: https://pkg.go.dev/github.com/gorilla/websocket (verified, last release Jun 2024)
- log/slog: https://pkg.go.dev/log/slog (verified, stdlib since Go 1.21)
- embed: https://pkg.go.dev/embed (verified, stdlib since Go 1.16)
- chi v5.2.5: https://pkg.go.dev/github.com/go-chi/chi/v5 (verified, Feb 2026)
- Go 1.26 release: https://go.dev/doc/devel/release (verified)
