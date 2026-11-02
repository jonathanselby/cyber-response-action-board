# CRAB build evidence — 0.0.26-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C5 (closure and reopening, contract addendum A1).
**Governing documents:** `docs/contract-addendum-v0.2.1.md` A1 (approved 2 November 2026); `docs/uat-remediation-triage.md` §3.9; `docs/remediation-build-plan.md` C5; `TEST-ACCEPTANCE-NOTES.md` §5 (T-228, T-229)
**Implementation commits:** `dfedc6f`, review fix `ecad4f1`

## Scope

| Item | Result | Tests |
|---|---|---|
| Closure refused while stand-down, PIR or retention are missing, any proposal is pending (every proposal table, the closure itself excluded) or an obligation clock is still running; each blocker named | done | T-228 |
| Close page lists "What stops closure now"; the closure form appears only when nothing is outstanding | done | T-228 |
| Governed reopening: `incident.reopening` by Propose→Decide; a reason is required; a different Major confirms; no time window covers it | done | T-229 |
| Closure and stand-down kept as superseded history and shown as such; closing again needs a new stand-down and closure | done | T-229 |
| Migration 019: the closed-incident ledger trigger admits only `incident.reopening`; projection replay applies the same rule | done | T-229 |
| Access that expired while the incident was closed stays expired after reopening | done | T-229 |
| Abuse suite covers `POST /incident/{id}/close/reopen` | done | T-224 |

Business rules sit in `domain` and `app`; the web page and `cmd/crab` adapter only display blockers and pass the form through. The policy-reference ratchet is unchanged.

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| Closure blockers named pending proposals and running clocks in every compartment, so a GENERAL-only Major could read the names of LEGAL clocks on the Close page or in the closure refusal | real defect (compartment leak) | fixed in `ecad4f1`: matters the reader cannot see are counted, not named; closure is still refused. Mutation check: removing the compartment filter fails `TestClosureBlockersIncludePendingProposalsAndRunningClocks` |
| Relationship proposals have no compartment | by design (relationships are GENERAL) | treated as GENERAL |

## Gates

| Gate | Build environment | GitHub CI (`ecad4f1`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
