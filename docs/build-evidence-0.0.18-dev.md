# CRAB build evidence — 0.0.18-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. UAT Phase 2 response, Stage 3 (organisation configuration).  
**Governing notes:** `UAT2-BUILD-NOTES.md` §4, `TEST-ACCEPTANCE-NOTES.md` §4

## Scope

| Item | Result | Tests |
|---|---|---|
| One governed organisation record family (SYSTEM proposals, one `/decide` kind) | done | integration: binding and rebuild |
| Severity scale confirmed at setup by both setup people; seed Severity level 1–5 | done | T-202 |
| Declarations and severity changes refuse values outside the confirmed scale; existing values still display | done | T-202 |
| Organisation screen: scale, emergency information, proposals | done | web + e2e |
| Emergency information fills the Emergency Response Pack and Incident Brief contacts | done | T-203 (PDF text checked with `pdftotext` where installed) |
| Pack becomes stale when emergency information is confirmed after the last pack | done | T-204 |
| Contacts readable by Majors and Custodians only; printing still needs `print` | done | T-203, app unit test |

## Data

- Migration 016 adds `crab.organisation_proposals` and `crab.organisation_records` (rebuildable projections). Schema version 16.
- `OrganisationProjection` joins the rebuild set. A decision must match the stored proposal's type, version, maker and payload.

## Decisions recorded during build

- Policy changes are proposed and confirmed by Majors. The maker can never confirm their own change.
- The emergency form is plain text: one entry per line, fields separated by `|`, procedures headed by `## `. It needs no JavaScript and round-trips the current configuration for editing.
- Pack freshness now looks at the Emergency Response Pack file specifically (it previously accepted any PDF) and compares it with the last confirmed emergency information.
- Screen count rises from 10 to 11 (Organisation), for UAT items 2, 5 and 6.

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 8/8 |
| `scripts/build.sh` linux/amd64 | pass; 17,977,607 bytes |
| `scripts/build.sh` linux/arm64 | pass |
