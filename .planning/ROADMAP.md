# Roadmap: Sofar HYD Diagnostic Tool

## Milestones

- ✅ **v1.0** -- Phases 1-5 (shipped 2026-04-11) -- [Archive](milestones/v1.0-ROADMAP.md)
- ✅ **v1.1 UX Polish & Battery Pack Fix** -- Phases 6-7 (shipped 2026-04-11) -- [Archive](milestones/v1.1-ROADMAP.md)
- ✅ **v1.2 Reliability & UX Refinements** -- Phases 8-11 (shipped 2026-04-13) -- [Archive](milestones/v1.2-ROADMAP.md)
- ✅ **v1.3 Data Cleanup & Configuration** -- Phases 12-17 (shipped 2026-04-14) -- [Archive](milestones/v1.3-ROADMAP.md)
- ✅ **v1.4 Batch Register Reading** -- Phases 18-19 (shipped 2026-04-15) -- [Archive](milestones/v1.4-ROADMAP.md)
- ✅ **v1.5 Full Batch Reading & Configuration Cleanup** -- Phases 20-26 (shipped 2026-04-18) -- [Archive](milestones/v1.5-ROADMAP.md)
- 🚧 **v1.6 CI/CD, Docker & Test Performance** -- Phases 27-31 (in progress)

## Phases

<details>
<summary>✅ v1.0 (Phases 1-5) -- SHIPPED 2026-04-11</summary>

- [x] Phase 1: Foundation and Modbus Service (3/3 plans)
- [x] Phase 2: WebSocket Hub, API, and Connection UI (3/3 plans)
- [x] Phase 3: Core Monitoring Sections (3/3 plans)
- [x] Phase 4: Battery Overview and Statistics (4/4 plans)
- [x] Phase 5: Deep Battery Pack Diagnostics (3/3 plans)

</details>

<details>
<summary>✅ v1.1 UX Polish & Battery Pack Fix (Phases 6-7) -- SHIPPED 2026-04-11</summary>

- [x] Phase 6: Battery Pack Access Fix (3/3 plans)
- [x] Phase 7: Streaming Display and Configurable Timing (3/3 plans)

</details>

<details>
<summary>✅ v1.2 Reliability & UX Refinements (Phases 8-11) -- SHIPPED 2026-04-13</summary>

- [x] Phase 8: Refresh Architecture (2/2 plans) -- completed 2026-04-12
- [x] Phase 9: Connection & Read Resilience (2/2 plans) -- completed 2026-04-12
- [x] Phase 10: Data Persistence & Tooltips (3/3 plans) -- completed 2026-04-13
- [x] Phase 11: Battery Pack Polish (2/2 plans) -- completed 2026-04-13

</details>

<details>
<summary>✅ v1.3 Data Cleanup & Configuration (Phases 12-17) -- SHIPPED 2026-04-14</summary>

- [x] Phase 12: Dead Code Cleanup & Test Infrastructure (2/2 plans)
- [x] Phase 13: Statistics-to-System Merge (1/1 plans)
- [x] Phase 14: System Time Fix (1/1 plans)
- [x] Phase 15: Configuration Section (3/3 plans)
- [x] Phase 16: Frontend Polish (2/2 plans)
- [x] Phase 17: XLSX Register Discovery (3/3 plans)

</details>

<details>
<summary>✅ v1.4 Batch Register Reading (Phases 18-19) -- SHIPPED 2026-04-15</summary>

- [x] Phase 18: Batch Read Infrastructure (2/2 plans) -- completed 2026-04-14
- [x] Phase 19: System & Configuration Batch Application (2/2 plans) -- completed 2026-04-15

</details>

<details>
<summary>✅ v1.5 Full Batch Reading & Configuration Cleanup (Phases 20-26) -- SHIPPED 2026-04-18</summary>

- [x] Phase 20: Configuration Register Cleanup (2/2 plans) -- completed 2026-04-15
- [x] Phase 21: Standard Section Batch Verification (2/2 plans) -- completed 2026-04-17
- [x] Phase 22: SpanTracker Integration (3/3 plans) -- completed 2026-04-18
- [x] Phase 23: Battery Section Batch Migration (2/2 plans) -- completed 2026-04-16
- [x] Phase 24: BMS Batch Migration (2/2 plans) -- completed 2026-04-17
- [x] Phase 25: Pack Drill-Down Batch Migration (2/2 plans) -- completed 2026-04-17
- [x] Phase 26: Milestone Cleanup (1/1 plan) -- completed 2026-04-17

</details>

### 🚧 v1.6 CI/CD, Docker & Test Performance (In Progress)

**Milestone Goal:** Production-ready CI/CD pipeline with Docker packaging, automated releases via conventional commits, and fast test suite.

- [x] **Phase 27: Hub Test Optimization** - Eliminate 160s hub test bottleneck before building CI around it (completed 2026-04-19)
- [x] **Phase 28: Docker Packaging** - Minimal container image as foundation for Docker-based releases (completed 2026-04-19)
- [x] **Phase 29: PR Workflow** - Automated quality gates on every pull request (completed 2026-04-19)
- [ ] **Phase 30: Release Automation** - Conventional-commit semver releases with Docker push to ghcr.io
- [ ] **Phase 31: Dependency Management** - Automated dependency update PRs via Dependabot

## Phase Details

### Phase 27: Hub Test Optimization
**Goal**: Hub test suite runs fast enough to be a useful CI gate, not a liability
**Depends on**: Nothing (first phase of v1.6)
**Requirements**: TEST-01, TEST-02, TEST-03, TEST-04
**Success Criteria** (what must be TRUE):
  1. `go test ./internal/hub/...` completes in under 60 seconds (down from 160s)
  2. Hub tests that do not share state run in parallel via t.Parallel()
  3. collectRawMessages/drainRawMessages helpers terminate on idle timeout, not fixed duration
  4. Time-dependent hub tests use testing/synctest where beneficial, eliminating real-time waits
**Plans:** 3/3 plans complete
Plans:
- [x] 27-01-PLAN.md -- Fix enforceInterReadDelay burst bug (D-07) + regression test
- [x] 27-02-PLAN.md -- Rewrite drain/collect helpers to idle-timeout + synctest.Test migration for all 72 tests
- [x] 27-03-PLAN.md -- Race analysis + t.Parallel() addition + final timing verification

### Phase 28: Docker Packaging
**Goal**: Project builds a minimal, secure Docker container image suitable for CI and local use
**Depends on**: Nothing (independent of Phase 27)
**Requirements**: DOCK-01, DOCK-02, DOCK-03, DOCK-04, DOCK-05
**Success Criteria** (what must be TRUE):
  1. `docker build .` produces a working image under 15MB using multi-stage build
  2. Container runs as non-root user on distroless/static-debian12:nonroot base
  3. .dockerignore prevents build artifacts, PDFs, Excel files, and planning files from entering the build context
  4. Binary is statically linked with CGO_ENABLED=0 (no libc dependency)
  5. `curl http://localhost:<port>/healthz` returns 200 OK from the running container
**Plans:** 2/2 plans complete
Plans:
- [x] 28-01-PLAN.md -- Version injection, env var overrides, and health/readiness/status endpoints
- [x] 28-02-PLAN.md -- Dockerfile, .dockerignore, and Makefile docker targets with container verification

### Phase 29: PR Workflow
**Goal**: Every pull request is automatically validated for build, test, and lint correctness
**Depends on**: Phase 27 (fast tests make CI practical), Phase 28 (validates Docker build works)
**Requirements**: CIPR-01, CIPR-02, CIPR-03, CIPR-04
**Success Criteria** (what must be TRUE):
  1. Opening a PR triggers parallel CI jobs for build, lint, fast tests, and hub tests
  2. A docs-only PR (changing only *.md or docs/) completes CI without running build/test/lint jobs
  3. Go module dependencies are cached between workflow runs (no 30s download per run)
  4. A PR with a compilation error or test failure shows a red check and cannot merge
**Plans:** 1/1 plans complete
Plans:
- [x] 29-01-PLAN.md -- GitHub Actions PR validation workflow with docs-skip, parallel jobs, and JUnit test reporting

### Phase 30: Release Automation
**Goal**: Merging to master automatically creates versioned releases with Docker images and binary artifacts
**Depends on**: Phase 28 (Dockerfile for Docker push), Phase 29 (CI validates before release)
**Requirements**: REL-01, REL-02, REL-03, REL-04
**Success Criteria** (what must be TRUE):
  1. Merging a `feat:` commit to master auto-creates a minor version tag (e.g., v1.7.0)
  2. GitHub Release is created with auto-generated changelog from conventional commit history
  3. Docker image is built and pushed to ghcr.io/<owner>/modbus_reader with semver + latest tags
  4. Compiled binary artifact is attached to the GitHub Release for direct download
**Plans**: TBD

### Phase 31: Dependency Management
**Goal**: Go module and GitHub Actions dependencies stay current via automated update PRs
**Depends on**: Phase 29 (Dependabot PRs need CI to validate them)
**Requirements**: DEP-01, DEP-02
**Success Criteria** (what must be TRUE):
  1. Dependabot opens weekly grouped PRs for Go module dependency updates
  2. Dependabot opens PRs for GitHub Actions version updates (setup-go, golangci-lint-action, etc.)
**Plans**: TBD

## Progress

**Execution Order:**
Phases execute in numeric order: 27 -> 28 -> 29 -> 30 -> 31

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 27. Hub Test Optimization | v1.6 | 3/3 | Complete    | 2026-04-19 |
| 28. Docker Packaging | v1.6 | 2/2 | Complete    | 2026-04-19 |
| 29. PR Workflow | v1.6 | 1/1 | Complete    | 2026-04-19 |
| 30. Release Automation | v1.6 | 0/0 | Not started | - |
| 31. Dependency Management | v1.6 | 0/0 | Not started | - |
