# CRAB build evidence — 0.0.22-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C1 (identity and credential core).
**Governing documents:** `docs/uat-remediation-triage.md` §0, §3.1, §3.3, §3.17; `docs/remediation-build-plan.md` C1; `TEST-ACCEPTANCE-NOTES.md` §5
**Implementation commits:** `e766e83` (implementation), `c477756` (review fix)

## Scope

| Item | Result | Tests |
|---|---|---|
| `security.identity` governed record: display name, organisation, external, active/suspended/disabled, account expiry, reason. No contact data (strict decode) | done | T-214 |
| Custodian maker, different usable Custodian confirmer; disabled is final and removes the person's credential material | done | T-214 |
| `access.Usable`, the one account rule, applied at sign-in and in `app.Principals` (the one place a session becomes a principal for browser, API, MCP and CLI print) | done | T-218 |
| Suspension or disablement deletes every session of that person in the ledger transaction | done | T-218 |
| Principal expiry evaluated against the governed clock; the wall-clock `AccessStore.Principal` removed | done | T-219 |
| Expired sessions swept on sign-in, at most 100 per sign-in | done | T-220 |
| Role eligibility after setup: Custodian maker, different confirmer, active identity record required | done | T-215 |
| Every access decision re-checks at confirmation that maker and confirmer are usable and the maker still holds the authority | done | T-248 |
| `security.credential_reset`: enrolment codes need two Custodians and are shown once to the confirmer only; hashed; 24-hour expiry; no session until a compliant password is set; reset ends sessions, old password and TOTP | done | T-216, T-217 |
| TOTP required for external people at enrolment and sign-in; account expiry required and at most 30 days | done | T-250 |
| Bootstrap writes identity records in the same atomic batch | done | ceremony test |
| Policy-reference ratchet over `internal/presentation/**` and `cmd/crab` | done | T-253 |
| Browser screens for people, enrolment and TOTP | not in C1 | C2 |

## Design notes and departures from the triage

1. **Credential reset is dual-controlled** (triage U-15 said one Custodian may reset). While building, it became clear that whoever sees a code can sign in as that person first, so a single Custodian could take over a Major's account and confirm their own proposals. The code is now issued only through a governed proposal, generated at confirmation and shown to the confirmer alone. A takeover would need two people working together. The product owner was told before the build continued.
2. **No back-fill on upgrade.** A migration cannot sign ledger records, and a back-filled projection row would vanish on rebuild. Accounts that hold role eligibility or a Custodian designation but have no identity record are treated as usable *legacy* accounts. New eligibility always requires an active identity record, so the legacy population cannot grow. New installations have no legacy accounts.
3. **Credential side effects run after the decision commits, not in the projection.** If a projection deleted TOTP rows, a ledger rebuild would replay old resets and destroy current enrolments. If the process stops between the commit and storing the code, history shows the reset and a new one can be proposed. The exception is session and (on disable) credential deletion on identity changes: replaying those is harmless.
4. **Session sweep on sign-in** replaces the planned timed-wait cadence. It is bounded and needs no scheduler.
5. **Enrolment is three app calls** (`Authenticate` → `BeginEnrolment` → `CompleteEnrolment`). The browser flow arrives in C2. Until then the existing sign-in form serves internal people, and external people cannot sign in through the browser: this fails closed.
6. **Policy-reference baseline** is 15 matches across 6 `cmd/crab` files plus 1 navigation hint in `internal/presentation/web/page.go`. The triage quoted 14 because it counted lines, and one line holds three references. The test counts matches.
7. The credential wrap key (`CRAB_CREDENTIAL_WRAP_KEY_FILE`, 32 bytes base64, private file) is now loaded. Without it, TOTP enrolment is refused visibly.

## Review gate

This was a self red-team review of the stable branch. No independent reviewer was requested.

| Finding | Classification | Action |
|---|---|---|
| A disabled person's password and TOTP material were kept indefinitely | real defect, in scope | fixed in `c477756`, asserted in T-214 |
| Browser sign-in has no failure rate limit (`authn.FailureLimiter` exists, unwired) | real, pre-existing, out of C1 scope | register F-8 → C3 |
| Response time reveals whether an account exists | pre-existing; PBKDF2 cost dominates | accepted, recorded |
| A TOTP code can be replayed inside its 30-second window | standard TOTP behaviour | accepted, recorded |

## Mutation check

Making `access.Usable` always return true fails `TestAccountStateEndsLiveSessions` and `TestUnusableCustodiansCannotCompleteGovernedChanges`. The tests detect the defect they guard against.

## Migration

`018_identities.sql`: `crab.identities` projection; `local_credentials.must_set_password`, `local_credentials.expires_at`; `totp_credentials.confirmed` (existing rows confirmed). Schema version 18.

## Gates

| Gate | Build environment | GitHub CI (`c477756`) |
|---|---|---|
| `gofmt -l ./cmd ./internal` | clean | pass |
| `go vet ./...` | pass | pass |
| `go test -race -count=1 ./...` (PostgreSQL 16 locally, 17 in CI) | pass | pass |
| `scripts/bafflewick-acceptance.sh` | 9/9 | pass |
| `scripts/visual-acceptance-preflight.sh` | pass | pass |

Build environment: Go 1.27.2 built from the official source tag; modules fetched directly with a local, uncommitted `go.work` mapping `golang.org/x` and `gopkg.in` modules to their GitHub mirrors (`docs/remediation-build-plan.md` §3.2).
