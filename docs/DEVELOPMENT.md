<!-- generated-by: gsd-doc-writer -->
# Development

Guide for setting up, building, and contributing to the Sofar HYD Diagnostic Tool.

## Local setup

1. **Clone the repository:**

   ```bash
   git clone git@github.com:richie-tt/sofar-HYD-diag.git
   cd sofar-HYD-diag
   ```

2. **Install Go 1.26 or later.** The `go.mod` file specifies `go 1.26`. Confirm your version:

   ```bash
   go version
   ```

3. **Install dependencies:**

   ```bash
   go mod download
   ```

   The project uses a small set of external dependencies (chi router, gorilla/websocket, excelize for XLSX parsing, Fyne for a desktop PoC, and testify for test assertions). All are managed via Go modules.

4. **Build the server binary:**

   ```bash
   make server
   ```

   This compiles `cmd/server/main.go` into a `server` binary with the frontend (HTML/JS/CSS) embedded via `go:embed`. No separate frontend build step is required.

5. **Run the server:**

   ```bash
   ./server -inverter-host <YOUR_INVERTER_IP>
   ```

   Open `http://localhost:8080` in a browser. See [CONFIGURATION.md](CONFIGURATION.md) for all CLI flags.

## Build commands

All build targets are defined in the project root `Makefile`:

| Command | Description |
|---------|-------------|
| `make` or `make server` | Build the main server binary (`./server`) from `cmd/server/` |
| `make discover` | Build the XLSX register discovery tool (`./xlsx-discover`) with the `xlsx_discover` build tag |
| `make test` | Run all tests across all packages (`go test ./...`) |
| `make test-discover` | Run XLSX discovery tool tests with verbose output and the `xlsx_discover` build tag |
| `make check-size` | Build the server binary and print its file size in bytes |
| `make clean` | Remove built binaries (`server`, `xlsx-discover`) |

### Building individual packages

To test compilation of a single internal package without building the full binary:

```bash
go build ./internal/modbus
go build ./internal/broker
go build ./internal/hub
go build ./internal/register
```

## Project structure

```
cmd/
  server/          Server entry point (main.go) — flag parsing, router, graceful shutdown
  fyne-poc/        Experimental Fyne desktop GUI proof of concept
internal/
  modbus/          Low-level Modbus TCP and RTU protocol implementation
  broker/          Connection serializer — single TCP connection, command queue, reconnect
  hub/             WebSocket hub — client management, section subscriptions, streaming reads
  register/        Register definitions — probes, probe groups, formatting, section layouts
web/
  handler.go       HTTP/WebSocket route setup (chi router, embedded static file serving)
  static/          Embedded frontend files (index.html, app.js, style.css)
tools/
  xlsx-discover/   XLSX register map parser — compares protocol spec against code definitions
  config-sweep/    Diagnostic tool for sweeping configuration registers
  section-sweep/   Diagnostic tool for sweeping section register ranges
```

## Code style

This project follows standard Go conventions enforced by the standard toolchain. No third-party linters or formatters are configured.

- **Formatting:** `gofmt` (the Go default). Run before committing:

  ```bash
  gofmt -l .
  ```

  Or to reformat in place:

  ```bash
  gofmt -w .
  ```

- **Vet:** Standard static analysis:

  ```bash
  go vet ./...
  ```

- **No custom linter configs** — there are no `.golangci.yml`, `.eslintrc`, `.prettierrc`, or `.editorconfig` files in the repository.

- **Naming conventions:**
  - PascalCase for exported identifiers: `ReadHoldingRegistersTCP`, `ProbeGroup`, `Hub`
  - camelCase for unexported identifiers: `transactionID`, `sectionResult`, `upgrader`
  - Verb-first function names: `Read*`, `Write*`, `Format*`, `Setup*`
  - Short names in tight loops: `i`, `n`, `b`

- **Import organization:** Standard Go convention — stdlib first, then external packages, then internal packages, separated by blank lines.

- **Comments:** `//` single-line style. Section markers use `// === Section Name ===`. Protocol details and known hardware behaviors are documented inline.

## Commit message conventions

The project uses a structured prefix convention for commit messages:

```
type(scope): description
```

Common types observed in the commit history:

| Prefix | Usage |
|--------|-------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `test` | Adding or updating tests |
| `docs` | Documentation changes |
| `chore` | Cleanup, dead code removal, tooling |
| `revert` | Reverting a previous change |

The scope is typically a phase or milestone number (e.g., `feat(22-03):`), or a descriptive label for the area of change. Commit messages are concise single-line descriptions.

## Branch conventions

- **Default branch:** `master`
- No formal branch naming convention is documented. Feature work appears on `master` directly, with worktree branches used for parallel agent work (prefixed `worktree-agent-`).

## PR process

No pull request template (`.github/PULL_REQUEST_TEMPLATE.md`) is present in the repository. See [CONTRIBUTING.md](../CONTRIBUTING.md) for contribution guidelines. No CI workflows are configured in `.github/workflows/`.

When contributing:

- Run `make test` and ensure all tests pass before submitting
- Run `gofmt -l .` to confirm formatting
- Run `go vet ./...` to check for static analysis issues
- Follow the existing commit message convention (`type(scope): description`)
- Keep commits focused on a single logical change

## Testing

Tests use Go's built-in `testing` package with `github.com/stretchr/testify` for assertions. Test files follow the `*_test.go` naming convention and are colocated with their source packages.

Run the full test suite:

```bash
make test
```

For verbose output on a specific package:

```bash
go test -v ./internal/hub/
go test -v ./internal/register/
go test -v ./internal/modbus/
go test -v ./internal/broker/
go test -v ./web/
```

For the XLSX discovery tool (requires build tag):

```bash
make test-discover
```

See [docs/ARCHITECTURE.md](ARCHITECTURE.md) for a description of how the packages relate to each other.
