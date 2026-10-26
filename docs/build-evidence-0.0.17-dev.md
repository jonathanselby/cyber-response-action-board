# CRAB build evidence — 0.0.17-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. UAT Phase 2 response, Stage 2 (Storm Blue).  
**Governing notes:** `UAT2-BUILD-NOTES.md` §3, `TEST-ACCEPTANCE-NOTES.md` §4

## Scope

| Item | Result | Tests |
|---|---|---|
| Storm Blue tokens, light and dark, with the U-5 contrast rules | done; `crab.css` 1,903 bytes (budget 2,048) | T-185, T-200 |
| Logo on sign-in and setup, PWA icons, favicon, Storm Blue theme colour | done | T-179 unchanged |
| Command Summary replaces the Incidents page at `/` | done | T-198, T-199 |
| Four screen patterns applied through shared templates | done | T-211 (human) |
| PDF restyle: CRAB masthead, metadata bar, tables, ruled forms, mono-safe | done | T-201 |
| Governed print and download permissions in Access admin | done (gap found, see below) | e2e |
| Visual capture: every screen, 4 viewports, light and dark, plus PDFs | done | review aid |

## Gap found and closed: print permission had no path

Building PDF preview showed that the separate `print` and `download` permissions (AC-66) could not be granted by any browser, API or CLI path: the access service refused every output-permission proposal. In the Phase 2 build no one could print or download evidence. Access admin now lets one Custodian propose granting or withdrawing print or download for a named person; a different Custodian confirms. Reading an incident still never implies output authority.

## Visual review method

`scripts/visual-capture.sh` seeds a representative Bafflewick picture through the real browser routes (`cmd/crab` end-to-end harness against PostgreSQL), renders 16 screens and screenshots them at 390, 768 and 1366 px in light and 1366 px dark with headless Chrome. The `visual-capture` workflow publishes the images and sample PDFs to the `ci/visual-capture` branch as a single orphan commit. Findings from the first capture fixed in this checkpoint:

- disclosure sections had no visible affordance; summaries now show `+` / `−` in Storm Blue;
- Readiness showed internal check names; names are now plain words;
- Approvals showed stored values such as `not_started`; they now read as words;
- Access admin listed GENERAL read-ins under *Restricted compartments*; they are no longer listed there;
- `LIVE` used the success (green) badge; it now uses the neutral Storm Blue badge.

Known shell limitation (go-app-shell v0.2 request): signed-out pages still reserve the navigation rail on wide screens, and the header shows the wordmark as text.

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 (includes end-to-end harness) | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 8/8 |
| `scripts/build.sh` linux/amd64 | pass; 17,940,743 bytes |
| `scripts/build.sh` linux/arm64 | pass |

## Carried into Stage 3

Setup now confirms the first severity scale and declarations select from it. The Organisation screen that edits the scale and the emergency configuration lands in 0.0.18-dev.
