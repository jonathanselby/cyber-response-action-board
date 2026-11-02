# CRAB build evidence — 0.0.24-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C3 (external access, live revocation and access-abuse proof).
**Governing documents:** `docs/uat-remediation-triage.md` §3.2, §3.3, §3.11, §3.17; `docs/remediation-build-plan.md` C3; `TEST-ACCEPTANCE-NOTES.md` §5
**Implementation commit:** `ccf2093`

## Scope

| Item | Result | Tests |
|---|---|---|
| External access defaults to, and never outlasts, the account end (proposal and confirmation) | done | T-222 |
| EXTERNAL in words: Board role holder, incident Access page, Approvals maker, Incident Brief | done | T-222 (Brief: renderer code; PDF text is font-encoded and not text-asserted) |
| Change a person's role from People (increase, reduction, removal) | done | T-223 |
| Live revocation across two browser sessions, the API and MCP: role reduction and increase, compartment read-out, incident removal, suspension, reactivation, disablement; in-flight form refused | done | T-223 |
| Stale tokens after logout refused on browser, API and MCP | done | T-226 |
| Abuse suite over every registered route × four attacker principals | done | T-224 |
| Compartment abuse by an incident member without read-in | done | T-224 |
| Route coverage: a new route without an abuse case fails the build | done | T-225 |
| Sign-in and enrolment failure limits (F-8) | done | `TestSignInFailuresAreLimited` |

## Security defect found and fixed (F-10)

The abuse suite found a real defect. A Major who was not on an incident could triage a first report as *associate with INC-SECRET*. That wrote a disposition naming a hidden incident, and the accept/refuse difference confirmed the incident existed. This breaks EDD §9.5 (*hidden means hidden*; error behaviour must not become an existence oracle), which is part of the non-negotiable security floor.

**Fix:** `ContinuityService.authoriseTriageTarget`. Associating needs record authority in the incident. *Propose declaration* needs read authority, or being the maker of that pending declaration. Every refusal is identical.

**Existing test changed:** `TestContinuityProductFlowThroughLedgerAndPostgres` (Bafflewick phase 1) linked the report using a test principal marked *unassigned*. In the running system the declarer is a member once the declaration is confirmed, so the fixture now uses that realistic principal. It also gained an assertion that the unassigned association is refused. The legitimate flow is still proven; nothing is weakened.

## Proof the suite can fail

The first green run was vacuous. The `POST /logout` case ran early with each attacker's token and signed them out, so later cases ran unauthenticated. Two mutations exposed it: removing incident read authorisation, and removing the sponsor check, both went undetected.

**Fixes:**
- logout runs anonymously only;
- before every case, every attacker session is asserted alive; a dead session fails the test.

**After the fix:**
- removing incident read authorisation produces 327 failures;
- removing the triage fix is caught.

## F-9 not built

F-9 proposed listing incident memberships on Access admin only for incidents the Custodian is on. The build showed that this contradicts EDD §9.6: the Custodian *administers identity and access metadata* and confirms access changes for incidents they are not on. It also contradicts an existing test (`TestAccessGovernanceSponsorControllerCustodianAndExpiry`). It was reverted and recorded as *not built*. The People section still counts rather than names other incidents (U-17). That leaves an inconsistency on one page, raised with the product owner.

## Review gate

This was a self red-team review of the stable branch.

| Finding | Classification | Action |
|---|---|---|
| The failure limiter keys on the connecting address; behind a shared proxy or NAT, users would share the per-source limit | accepted: CRAB is served directly (README: non-loopback requires TLS on CRAB itself) | recorded |
| MCP coverage uses the runtime methods its adapter calls, not the JSON-RPC parser | accepted limitation | recorded |
| External marking adds one small query per operating-picture and Approvals load | measure in C11 | recorded |

## Gates

| Gate | Build environment | GitHub CI (`ccf2093`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| policy-reference ratchet | unchanged baseline | pass |
| Bafflewick | 9/9 | pass |
| visual preflight | pass | pass |
