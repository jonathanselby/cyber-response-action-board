# CRAB build evidence — 0.0.19-dev

**Date:** 26 October 2026  
**Status:** development checkpoint. UAT Phase 2 response, Stage 4 (related incidents and checklist templates).  
**Governing notes:** `UAT2-BUILD-NOTES.md` §5, `TEST-ACCEPTANCE-NOTES.md` §4

## Scope

| Item | Result | Tests |
|---|---|---|
| Governed relationship *A is related to B* with reason and time | done | T-205, T-206 |
| Proposing, confirming and seeing a link all require authority over both incidents | done | T-205 |
| Refusal for a hidden incident is identical to an unknown incident | done | T-205 |
| No access, history or compartment is shared; withdrawal is a new version | done | T-206 |
| Governed checklist templates in Organisation (propose, confirm, withdraw) | done | T-207 |
| *Activate checklist* on the Board: one proposed action per step with template provenance | done | T-207 |
| Owners from the step's default role on the incident, else the person activating | done | T-207 |
| Approvals groups the steps of one activation under one heading | done | T-207 |

## Data

- Migration 017: `crab.incident_relationship_proposals` and `crab.incident_relationships` (both directions). Schema version 17.
- Checklist templates use the Stage 3 organisation records (`system.checklist_template`, keyed by template identifier).

## Observed product gap (not solved here)

CRAB has no browser, API or CLI path to create further local accounts or grant role eligibility after first-run setup: role-eligibility proposals are refused at decision. A deployment therefore has exactly the two setup people. The UAT script works within that. This is routed to the product track (PRD access administration) rather than built in this response.

## Gates run in the build environment

| Gate | Result |
|---|---|
| `gofmt -l ./cmd ./internal` | clean |
| `go vet ./...` | pass |
| `go test -race -count=1 ./...` with PostgreSQL 16 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass |
| `scripts/bafflewick-acceptance.sh` | 8/8 |
| `scripts/build.sh` linux/amd64 | pass; 18,047,239 bytes |
| `scripts/build.sh` linux/arm64 | pass |
