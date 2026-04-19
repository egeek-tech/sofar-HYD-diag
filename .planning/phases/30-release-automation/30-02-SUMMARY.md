---
phase: 30-release-automation
plan: 02
subsystem: infra
tags: [release-please, docker, multi-arch, buildx, ci-cd, github-actions, ghcr]

# Dependency graph
requires: [30-01]
provides:
  - "Complete release.yml workflow with 3 jobs: release-please, docker, binaries"
  - "Automated semver releases from conventional commits on master merge"
  - "Multi-arch Docker image push to ghcr.io with semver+latest tags"
  - "Cross-compiled binary tar.gz artifacts attached to GitHub Releases"
affects: []

# Tech tracking
tech-stack:
  added: [googleapis/release-please-action@v4, docker/build-push-action@v7, docker/metadata-action@v6, docker/setup-buildx-action@v4, docker/setup-qemu-action@v4, docker/login-action@v4]
  patterns: [output-gated-job-chaining, matrix-cross-compilation, concurrency-no-cancel]

key-files:
  created:
    - .github/workflows/release.yml
  modified: []

key-decisions:
  - "type=raw tags for Docker metadata instead of type=semver (simpler, avoids undocumented value= parameter edge cases)"
  - "cancel-in-progress: false for release workflow (never cancel a release mid-flight)"
  - "CGO_ENABLED as string '0' in YAML env block (bare 0 is YAML integer, env vars must be strings)"
  - "GH_TOKEN via env var not CLI arg (prevents token leaking in logs)"

patterns-established:
  - "Output-gated job chaining: release-please outputs gate downstream docker and binaries jobs"
  - "Matrix cross-compilation: strategy.matrix.include for linux/amd64 and linux/arm64"

requirements-completed: [REL-01, REL-02, REL-03, REL-04]

# Metrics
duration: 1min
completed: 2026-04-19
---

# Phase 30 Plan 02: Release Workflow Summary

**GitHub Actions release.yml with release-please semver automation, multi-arch Docker push to ghcr.io, and cross-compiled binary tar.gz upload to GitHub Releases**

## Performance

- **Duration:** 1 min
- **Started:** 2026-04-19T19:15:15Z
- **Completed:** 2026-04-19T19:16:30Z
- **Tasks:** 1
- **Files modified:** 1

## Accomplishments
- Created .github/workflows/release.yml with 3 jobs and 100 lines of workflow configuration
- Job 1 (release-please): Parses conventional commits, manages release PRs, creates GitHub Releases with semver tags
- Job 2 (docker): Builds multi-arch images (linux/amd64,linux/arm64) via buildx+QEMU and pushes to ghcr.io with v-prefixed semver and latest tags
- Job 3 (binaries): Cross-compiles Go binaries via matrix strategy, packages as tar.gz, uploads to GitHub Release via gh CLI
- All action version pins match pr.yml exactly (checkout@v6.0.2, setup-go@v6.4.0, go-version-file: go.mod)
- Permissions scoped to minimum per D-10: contents:write, pull-requests:write, packages:write
- Anti-patterns avoided: no releases_created (plural), no google-github-actions (archived), no hardcoded secrets in run commands

## Task Commits

Each task was committed atomically:

1. **Task 1: Create release.yml workflow with release-please, Docker push, and binary upload jobs** - `d04b51c` (feat)

## Files Created/Modified
- `.github/workflows/release.yml` - Complete release automation workflow (3 jobs: release-please, docker, binaries)

## Decisions Made
- Used `type=raw` tags for Docker metadata-action instead of `type=semver` (per RESEARCH.md Open Question 1 resolution -- simpler and avoids undocumented edge cases)
- Set `cancel-in-progress: false` on concurrency group (unlike pr.yml which uses true -- release workflows must never be cancelled mid-flight)
- CGO_ENABLED set as string `'0'` in YAML env block (YAML treats bare 0 as integer; environment variables must be strings)
- GH_TOKEN passed via `env:` block not CLI argument (prevents token from appearing in process listing or logs per RESEARCH.md security domain)

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None

## User Setup Required
- Repository must have GitHub Actions enabled
- GITHUB_TOKEN automatic permissions must include packages:write (default for repos with GitHub Packages enabled)
- Branch protection on master should allow release-please to create/merge release PRs

## Next Phase Readiness
- Release pipeline is complete: push conventional commit to master -> release-please creates release PR -> merge PR -> Docker + binary artifacts produced automatically
- First `feat:` or `fix:` commit merged to master after this workflow is active will trigger release-please to create a v1.7.0 release PR
- All four REL requirements (REL-01 through REL-04) are implemented in this single workflow file

## Self-Check: PASSED

All files exist, all commits verified.

---
*Phase: 30-release-automation*
*Completed: 2026-04-19*
