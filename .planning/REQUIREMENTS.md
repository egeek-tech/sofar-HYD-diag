# Requirements: Sofar HYD Diagnostic Tool

**Defined:** 2026-04-19
**Core Value:** Provide clear, real-time visibility into all Sofar HYD inverter parameters through a reliable web interface

## v1.6 Requirements

Requirements for CI/CD, Docker packaging, and test performance milestone.

### Docker Packaging

- [ ] **DOCK-01**: Project has a multi-stage Dockerfile producing a minimal container image
- [ ] **DOCK-02**: .dockerignore excludes build artifacts, planning files, and dev tooling
- [ ] **DOCK-03**: Container runs as non-root user with distroless/static base
- [ ] **DOCK-04**: Binary built with CGO_ENABLED=0 for fully static linking
- [ ] **DOCK-05**: /health endpoint returns 200 OK for container liveness checks

### PR Workflow

- [ ] **CIPR-01**: GitHub Actions workflow runs build, test, and golangci-lint on every PR
- [ ] **CIPR-02**: Workflow skips execution on docs-only changes (*.md, docs/)
- [ ] **CIPR-03**: Jobs run in parallel (lint, fast tests, hub tests, build)
- [ ] **CIPR-04**: Go module dependencies are cached between runs

### Release Automation

- [ ] **REL-01**: Merges to master auto-generate semver tags from conventional commits (feat->minor, fix->patch)
- [ ] **REL-02**: GitHub Release created with auto-generated changelog from commit history
- [ ] **REL-03**: Docker image built and pushed to ghcr.io on release
- [ ] **REL-04**: Binary artifact attached to GitHub Release

### Dependency Management

- [ ] **DEP-01**: Dependabot configured for Go module updates on weekly schedule
- [ ] **DEP-02**: Dependabot configured for GitHub Actions version updates

### Test Performance

- [ ] **TEST-01**: Hub test helpers (collectRawMessages/drainRawMessages) rewritten to eliminate unnecessary waits
- [ ] **TEST-02**: Hub tests use t.Parallel() where tests don't share state
- [ ] **TEST-03**: Hub test suite completes in under 60 seconds
- [ ] **TEST-04**: Hub tests migrated to testing/synctest where beneficial for time-dependent tests

## Future Requirements

Deferred to future release. Tracked but not in current roadmap.

### Batch Diagnostics

- **BDIAG-01**: API endpoint exposing batch plan span statistics per section
- **BDIAG-02**: Gap-filling reads for sparse register ranges

### Dynamic Discovery

- **DISC-01**: Runtime register auto-discovery for hardware-specific register support

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| GoReleaser cross-compilation | Single binary, single platform — Docker is the distribution format |
| Kubernetes manifests / Helm chart | Local diagnostic tool, not a cloud service |
| Code coverage enforcement | Not needed for a personal diagnostic tool |
| E2E integration tests in CI | Requires real inverter hardware, not possible in CI |
| Mobile Docker image variants | Desktop-only tool |
| Cross-section batch merging | Sections are independent streaming units; merging adds complexity for marginal gain |
| Dynamic register auto-discovery | Register map is fixed per hardware model; static removal is correct and simpler |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| DOCK-01 | Phase 28 | Pending |
| DOCK-02 | Phase 28 | Pending |
| DOCK-03 | Phase 28 | Pending |
| DOCK-04 | Phase 28 | Pending |
| DOCK-05 | Phase 28 | Pending |
| CIPR-01 | Phase 29 | Pending |
| CIPR-02 | Phase 29 | Pending |
| CIPR-03 | Phase 29 | Pending |
| CIPR-04 | Phase 29 | Pending |
| REL-01 | Phase 30 | Pending |
| REL-02 | Phase 30 | Pending |
| REL-03 | Phase 30 | Pending |
| REL-04 | Phase 30 | Pending |
| DEP-01 | Phase 31 | Pending |
| DEP-02 | Phase 31 | Pending |
| TEST-01 | Phase 27 | Pending |
| TEST-02 | Phase 27 | Pending |
| TEST-03 | Phase 27 | Pending |
| TEST-04 | Phase 27 | Pending |

**Coverage:**
- v1.6 requirements: 19 total
- Mapped to phases: 19
- Unmapped: 0

---
*Requirements defined: 2026-04-19*
*Last updated: 2026-04-19 after roadmap creation*
