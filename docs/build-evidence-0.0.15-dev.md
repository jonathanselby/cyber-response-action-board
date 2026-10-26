# CRAB build evidence — 0.0.15-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. Slice 15 browser presentation rebuild, ready for Phase 2 human UAT.  
**Governing notes:** `UI-SHELL-BUILD-NOTES.md`, `docs/go-app-shell-dependency-decision.md`, `TEST-ACCEPTANCE-NOTES.md` §3

## Scope

Phase 1 human UAT failed AC-43 (web visual quality). This checkpoint rebuilds the browser presentation tier on `go-app-shell` v0.1.0 and exposes application services that were already wired but unreachable from the browser. PRD V0.2 and EDD V0.4 are unchanged.

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` | pass (PostgreSQL integration tests skipped: no server in build environment) |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/build.sh` linux/amd64 | pass; binary 17,486,087 bytes (0.0.14-dev: 17,219,847) |
| `scripts/build.sh` linux/arm64 | pass |
| Real-browser check (Chrome for Testing 153) at 390 px and 1366 px, light and dark | Board, Approvals, First reports, Evidence and Close-out reviewed; two defects found (key/value grid markup, vertical rhythm) and fixed before commit |

**Must pass in GitHub Actions before UAT:** PostgreSQL integration tests, Bafflewick 8/8 and the module contract (`go mod tidy` against the public checksum database for the new `go-app-shell` entry).

## Size and budgets

| Measure | Value |
|---|---|
| Web package Go (non-test) | 2,277 lines (0.0.14-dev: about 2,430), now including Approvals, Close-out, paper downloads and seven newly exposed services |
| Web templates (moved out of Go strings) | 798 lines |
| `crab.css` | 1,046 bytes (budget 2 KiB) |
| CRAB-authored JavaScript | 0 bytes |
| Browser decide routes | 1 (was 6) |
| Runtime interfaces | 1 (was 6 optional) |
| Direct runtime dependencies | pgx, gopdf, x/image, go-app-shell |

## Build-environment note

The build sandbox's egress policy blocked `proxy.golang.org`, `go.dev` and vanity import hosts. Go 1.27.2 was built from the official `github.com/golang/go` tag. Modules were fetched directly from their GitHub sources and verified against the committed `go.sum`. A local workspace file outside the repository mapped `golang.org/x/*` to their GitHub mirrors at the pinned versions. Nothing about this is committed; CI resolves modules normally.

## Open evidence

- Phase 2 human UAT: T-187 to T-192 and the `docs/visual-acceptance.md` review.
- Raspberry Pi 5 physical resource run (unchanged from 0.0.14-dev).
