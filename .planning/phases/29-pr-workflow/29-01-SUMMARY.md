---
phase: 29-pr-workflow
plan: 01
subsystem: ci
tags: [github-actions, ci, pr-validation, lint, test, docker]
dependency_graph:
  requires: []
  provides: [pr-validation-workflow, docs-skip-filter, junit-test-reporting]
  affects: [.github/workflows/pr.yml]
tech_stack:
  added: [dorny/paths-filter@v4, dorny/test-reporter@v3, golangci-lint-action@v7, gotestsum, actions/setup-go@v5, actions/checkout@v6]
  patterns: [filter-gate, junit-reporting, concurrency-groups, minimal-permissions]
key_files:
  created: [.github/workflows/pr.yml]
  modified: []
decisions:
  - "Concurrency group with cancel-in-progress saves runner minutes on superseded pushes"
  - "Plain docker build command over docker/build-push-action (no buildx/QEMU needed for validation)"
  - "gotestsum --junitfile with test-reporter if:always() ensures annotations on failure"
  - "No checkout in changes job -- paths-filter uses GitHub API on pull_request events"
metrics:
  duration: 83s
  completed: "2026-04-19T16:19:44Z"
  status: complete
  tasks_completed: 2
  tasks_total: 2
---

# Phase 29 Plan 01: PR Validation Workflow Summary

GitHub Actions PR workflow with 5-job fan-out (changes, lint, test, build, docker) gated by dorny/paths-filter docs-skip, JUnit test annotations via gotestsum + test-reporter, and concurrency groups to cancel superseded runs.

## Task Results

### Task 1: Create GitHub Actions PR validation workflow (COMPLETE)

**Commit:** 7ff0031

Created `.github/workflows/pr.yml` with:

- **Trigger:** `pull_request` on `[master]` with `[opened, synchronize, reopened]` types
- **Concurrency:** `pr-${{ github.event.pull_request.number }}` with `cancel-in-progress: true`
- **Permissions:** Minimal -- `contents: read`, `pull-requests: read`, `checks: write`
- **Job 1 (changes):** `dorny/paths-filter@v4` with `src` and `docker` output filters, no checkout step
- **Job 2 (lint):** `actions/setup-go@v5` + `golangci-lint-action@v7` (v2.11), conditional on `src` filter
- **Job 3 (test):** `gotestsum --junitfile test-results.xml` + `dorny/test-reporter@v3` with `if: always()`, conditional on `src` filter
- **Job 4 (build):** `go build ./cmd/server`, conditional on `src` filter
- **Job 5 (docker):** `docker build --build-arg VERSION=ci-${{ github.sha }}`, conditional on `docker` filter
- All Go jobs use `setup-go@v5` with `go-version-file: go.mod` for version resolution and built-in module caching

All 20 acceptance criteria verified via automated grep checks.

### Task 2: Verify PR workflow on real pull request (COMPLETE)

**PR:** https://github.com/richie-tt/sofar-HYD-diag/pull/1

Verified all four CIPR requirements on a real PR:

1. **CIPR-01:** All 5 jobs (changes, lint, test, build, docker) appeared and passed
2. **CIPR-02:** Workflow-only push correctly skipped lint/test/build/docker (paths-filter working)
3. **CIPR-03:** lint, test, build, docker ran in parallel after changes completed
4. **CIPR-04:** setup-go handled Go module caching automatically

**Fixes applied during verification:**
- golangci-lint upgraded from v2.1 to v2.11 (v2.1 was built with Go 1.24, incompatible with Go 1.26)
- cmd/fyne-poc removed (unused GUI experiment with system deps unavailable on CI runners)

## Deviations from Plan

- golangci-lint version changed from v2.1 (D-08) to v2.11 — v2.1 was built with Go 1.24 and cannot analyze Go 1.26 code
- cmd/fyne-poc removed — build failed on CI due to missing system GUI libraries (libGL)

## Known Stubs

None -- workflow file is complete and self-contained.

## Decisions Made

1. Used `${{ github.sha }}` for Docker VERSION build-arg (safe -- Git SHA is not user-controllable input)
2. Added `.dockerignore` to the docker paths-filter (matches RESEARCH.md complete example, not just plan minimum)
3. Test timeout set to 120s as safety net per plan specification

## Verification

All automated verification checks passed:
- File contains all 5 jobs with correct names
- All conditional gates use correct `needs.changes.outputs` expressions
- No `-race` flag present
- `if: always()` on test-reporter step confirmed
- 3 instances of `go-version-file: go.mod` (lint, test, build)
- Concurrency group with cancel-in-progress configured

## Self-Check: PASSED

- [x] `.github/workflows/pr.yml` exists (FOUND)
- [x] Commit 7ff0031 exists (FOUND)
