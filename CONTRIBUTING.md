<!-- generated-by: gsd-doc-writer -->
# Contributing to Sofar HYD Diagnostic Tool

Thank you for your interest in contributing. This document covers the essentials for getting your changes accepted.

## Development Setup

See [README.md](README.md) for prerequisites (Go 1.26+, network access to an inverter) and build instructions. For detailed architecture context, see [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). For environment variable and configuration details, see [docs/CONFIGURATION.md](docs/CONFIGURATION.md).

Quick setup summary:

```bash
git clone git@github.com:egeek-tech/sofar-HYD-diag.git
cd sofar-HYD-diag
make server
```

## Pre-commit Hooks

This project uses [pre-commit](https://pre-commit.com/) to run automated checks before each commit. Install it once after cloning:

```bash
pre-commit install
```

The following hooks run automatically on `git commit`:

| Hook | Source | What it does |
|------|--------|--------------|
| `go-fmt` | dnephin/pre-commit-golang | Checks `gofmt` formatting |
| `golangci-lint` | dnephin/pre-commit-golang | Runs golangci-lint static analysis |
| `go-build` | dnephin/pre-commit-golang | Verifies the project compiles |
| `go-mod-tidy` | dnephin/pre-commit-golang | Ensures `go.mod`/`go.sum` are tidy |
| `go-unit-tests` | local | Runs `go test ./...` |
| `go-mod-verify` | local | Runs `go mod verify` |
| `trailing-whitespace` | pre-commit-hooks | Removes trailing whitespace |
| `end-of-file-fixer` | pre-commit-hooks | Ensures files end with a newline |
| `check-yaml` | pre-commit-hooks | Validates YAML syntax |
| `check-added-large-files` | pre-commit-hooks | Prevents committing large files |
| `hadolint` | local | Lints Dockerfiles (if present) |

If a hook fails, the commit is aborted. Fix the issue and re-run `git commit`. To run all hooks manually against the full repo:

```bash
pre-commit run --all-files
```

## Coding Standards

- **Formatting:** All Go code must be formatted with `gofmt`. The `go-fmt` pre-commit hook enforces this automatically.
- **Linting:** `golangci-lint` runs as a pre-commit hook. Fix any reported issues before committing.
- **Testing:** Run the full test suite with `make test` (which executes `go test ./...`). New functionality should include tests. The xlsx-discover tool has its own test target: `make test-discover`.
- **Naming conventions:** Follow the patterns established in the codebase -- PascalCase for exported identifiers, camelCase for unexported ones. See `CLAUDE.md` for detailed naming guidance.

## Commit Message Format

This project follows conventional commit style. Every commit message should use the format:

```
type(scope): short description
```

Common types used in this project:

| Type | Purpose |
|------|---------|
| `feat` | New feature or capability |
| `fix` | Bug fix |
| `test` | Adding or updating tests |
| `docs` | Documentation changes |
| `chore` | Maintenance, cleanup, dependency updates |
| `revert` | Reverting a previous change |

The scope typically references a phase number or module area (e.g., `fix(26): description` or `test(22-03): description`). Keep the description concise and lowercase.

## Branch Conventions

- The default branch is `master`.
- No formal branch naming convention is documented. Use descriptive branch names that indicate the purpose of the change (e.g., `fix-battery-reconnect`, `add-pack-drill-down`).

## PR Guidelines

- Keep pull requests focused on a single concern. Avoid mixing unrelated changes.
- Ensure all tests pass (`make test`) before submitting.
- Run `gofmt` on all modified Go files.
- Write a clear PR description explaining what the change does and why it is needed.
- If the change affects Modbus communication behavior, note any timing considerations (500ms inter-register delay, 1s BMS pack-switch settle time).
- Reference any related issues in the PR description.

## Issue Reporting

No issue templates are configured. When reporting a bug, please include:

- **Steps to reproduce** the problem
- **Expected behavior** vs **actual behavior**
- **Environment details:** Go version, OS, inverter model and firmware version if relevant
- **Log output** showing any `[FAIL]` or `[ERROR]` messages from the server

For feature requests, describe the use case and how the proposed change would help diagnose or monitor the inverter.

## Project Structure

The codebase is organized into these top-level directories:

| Directory | Purpose |
|-----------|---------|
| `cmd/server` | Main server entry point |
| `internal/modbus` | Modbus TCP/RTU protocol implementation |
| `internal/hub` | WebSocket hub, batch reading, client management |
| `internal/broker` | Message broker between Modbus and WebSocket layers |
| `internal/register` | Register definitions, section schema, batch planning |
| `web/` | HTTP handlers and embedded static frontend |
| `tools/` | Auxiliary tools (xlsx-discover, config-sweep, section-sweep) |

## License

This project is licensed under the [GNU General Public License v3.0](LICENSE). By contributing, you agree that your contributions will be licensed under the same terms.
