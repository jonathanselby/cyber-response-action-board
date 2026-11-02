# CRAB build evidence — 0.0.27-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C6 (machine first-report intake, contract addendum A4).
**Governing documents:** `docs/contract-addendum-v0.2.1.md` A4 (approved 2 November 2026); `docs/uat-remediation-triage.md` §3.6; `docs/remediation-build-plan.md` C6; `TEST-ACCEPTANCE-NOTES.md` §5 (T-230); `docs/first-report-api.md`
**Implementation commits:** `8836806`, review fix `28cea5a`

## Scope

| Item | Result | Tests |
|---|---|---|
| `security.intake_key`: issue, rotate and revoke, each governed by two Custodians; bound to one accountable person; the name is never reused; expiry is required (at most 365 days) | done | T-230 |
| The secret is shown once to the confirmer and stored as SHA-256 in supporting state (`crab.intake_key_secrets`). It is never in the ledger | done | T-230 |
| Rotation overlap of 0 to 24 hours; the previous secret stops at the end of the overlap; revocation deletes the secrets | done | T-230 |
| `POST /api/v1/first-reports`: JSON only, 64 KiB, TLS or loopback, no attachments; an ordinary first report with the key as source and the accountable person as maker; never an incident without triage | done | T-230 |
| Per-key `external_ref` idempotency, enforced by a unique index, so a concurrent repeat cannot commit twice | done | T-230 |
| 60 a minute per key, then `429` with nothing written and no queue; refused presentations are limited per source | done | T-230 |
| Identical refusal for expired, revoked, rotated-out, wrong, unknown and malformed keys, a person's session token, and a key whose accountable person cannot act | done | T-230 |
| Access admin *Intake keys* section; Approvals confirms through the action that shows the secret | done | T-230 |
| People names every incident a person belongs to (U-17 resolved: names) | done | `TestPeopleSectionRendersWhatTheRuntimeDecided` |
| `docs/first-report-api.md` | done | — |

No new store, scheduler, queue or independent principal: an intake key is never a session or a principal (it is refused on the browser, the API reads and writes, and MCP), and reports are recorded with the accountable person as maker. The rules are in `security/access` and `app`; the web handler maps the outcome the runtime returns to HTTP. The policy-reference ratchet is unchanged.

## Mutation checks

Each rule was removed in turn. `TestMachineIntakeKey` fails every time:

| Rule removed | Failure |
|---|---|
| Accountable person must be usable | the key of a suspended person is accepted |
| Previous secret ends at the end of the rotation overlap | the rotated-out secret is accepted |
| Per-key rate bound | the 61st submission is recorded |
| Key expiry | the expired key is accepted |

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| The rate bound used `Allow` then `Failure`, so concurrent senders could exceed 60 a minute | real defect | fixed in `28cea5a`: `FailureLimiter.Take` admits and counts under one lock. `TestTakeIsAStrictBoundUnderConcurrency` sends 500 goroutines and admits exactly 60 |
| An unknown key name was refused after one query and a known name after two, so names could be found by timing | real defect (minor) | fixed in `28cea5a`: the secret is checked first, so both are refused after one query |
| After 50 refused presentations from one address, valid keys from that address are also refused for 15 minutes | by design (stops guessing); documented in `docs/first-report-api.md` | recorded |
| Secrets of expired keys stay in supporting state until the key is revoked | they can never authenticate, because expiry is checked and an expired key cannot be rotated | recorded; no change |

## Gates

| Gate | Build environment | GitHub CI (`28cea5a`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
