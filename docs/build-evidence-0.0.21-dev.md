# CRAB build evidence — 0.0.21-dev

**Date:** 26 October 2026  
**Status:** release candidate for Phase 3 UAT. UAT Phase 2 response complete (Stages 1–6).  
**Governing notes:** `UAT2-BUILD-NOTES.md`, `TEST-ACCEPTANCE-NOTES.md` §4, `docs/uat-phase3-script.md`

## Phase 2 concerns and where they were addressed

| Phase 2 concern | Stage | Version | Tests |
|---|---|---|---|
| Password needs upper, lower and special symbol | 1 | 0.0.16-dev | T-193 |
| Time authority (one clock, TIME UNVERIFIED) | 1 | 0.0.16-dev | T-194 to T-196 |
| Simpler print preview showing the PDF | 1 | 0.0.16-dev | T-197 |
| Storm Blue UX overhaul, logo, dark mode, print restyle | 2 | 0.0.17-dev | T-200, T-201, T-211, T-212 |
| Summary page after sign-in (Command Summary) | 2 | 0.0.17-dev | T-198, T-199, T-213 |
| Governed severity values (Severity level 1–5) | 3 | 0.0.18-dev | T-202 |
| Organisation emergency configuration and real paper pack | 3 | 0.0.18-dev | T-203, T-204 |
| Related incidents without access leakage | 4 | 0.0.19-dev | T-205, T-206 |
| Governed checklist templates | 4 | 0.0.19-dev | T-207 |
| Single-person emergency approval window | 5 | 0.0.20-dev | T-208 to T-210 |

## Stage 6 work

- `docs/uat-phase3-script.md`: a 90-minute script for two people (or two browser sessions) that walks every Phase 2 concern, states what to expect, and lists the rows to score.
- `docs/visual-acceptance.md`: duplicated Phase 2 record merged; *Phase 3 human UAT — PENDING* added with T-187 to T-192 and T-211 to T-213.
- Demo seed (used by the visual capture) now opens a single-person window and records one self-approved decision, so the reference renders show the SELF-APPROVAL markings and the window table.
- Current-state documents updated to Bafflewick 9/9 (README, final-acceptance progress, Pi acceptance, `scripts/final-acceptance.sh`).

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 9/9 |
| `scripts/build.sh` linux/amd64 | pass; 18,100,487 bytes |
| `scripts/build.sh` linux/arm64 | pass; 16,777,376 bytes |

## Self-check of the reference renders

The `ci/visual-capture` branch holds screenshots of 18 views at 1366 px (light and dark), 768 px and 390 px, plus the Brief in colour and monochrome and the Pack on Letter. Checked before handing to UAT: Storm Blue tokens in light and dark, status colours only on state badges, the logo on sign-in and setup, the Command Summary listing only authorised items, the Organisation screen tables, and the PDF masthead and ruled forms. Found and fixed: PDF footers in developer builds and the reference renders said `config 0.0.15-dev` because the `buildinfo` default had not followed `VERSION`; the default now matches and `TestDefaultVersionMatchesVERSIONFile` keeps it so (release builds already stamped the right value through `scripts/build.sh`). Observed, not changed: at 390 px the header status text is truncated with an ellipsis (go-app-shell header); the full status remains in the Board warning and the Command Summary. This check is not a substitute for T-211 to T-213.

## Known limits carried into Phase 3 UAT

- Only the two setup people: no path yet to add accounts or role eligibility after setup (product track).
- Header logo slot and the signed-out navigation rail depend on go-app-shell v0.2.
- The time check reads the host's synchronisation state; CRAB does not query time servers itself.
- Raspberry Pi 5 physical resource run (AC-58) and human visual sign-off (AC-43) remain the two empirical release gates.
