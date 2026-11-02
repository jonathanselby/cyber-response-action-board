# CRAB build evidence — 0.0.23-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C2 (user administration in the browser).
**Governing documents:** `docs/uat-remediation-triage.md` §0, §3.1, §3.17; `docs/remediation-build-plan.md` C2; `TEST-ACCEPTANCE-NOTES.md` §5
**Implementation commits:** `a2ad582` (implementation), `e8bdfe9` (review fix)

## Scope

| Item | Result | Tests |
|---|---|---|
| Access admin *People*: status, EXTERNAL, role, Custodian, account end, incidents (named only where the Custodian may see them, otherwise counted; U-17) | done | web rendering test |
| Add a person: identity and role proposed together, validated before either is written, grouped in Approvals as *New person · id* | done | T-221, `TestNewPersonIsAllOrNothing` |
| Suspend, reactivate, disable, request enrolment code; each needs a second Custodian | done | T-221 |
| Approvals words identity, role and enrolment-code proposals from `app.AccessApprovalSummary` | done | T-221 |
| Confirming an enrolment code shows it once, to the confirmer only, in the response body, on a no-store page; the maker is never offered it | done | T-221, web test |
| Sign-in accepts an enrolment code (sends to set-up) and an authenticator code | done | T-221, web test |
| First-time set-up page; authenticator key and code step only for external people | done | T-221, web test |
| Readiness warns below three usable Custodians | done | T-249 |
| The 02:17 journey (brief R-01 steps 1–10) end to end through the browser runtime | done | T-221 |

## Journey coverage (T-221)

| Brief step | Proven |
|---|---|
| 1–3 create Jane, make her eligible, activate | external Minor created by alice, confirmed by bob; enrolment code via alice → bob |
| 4 Sponsor assigns INC-004 | alice (declaring sponsor) proposes, Custodian bob confirms |
| 5 Controller adds FORENSICS | controller designation, then read-in, each confirmed |
| 6 access expires in six hours | membership and read-in carry a six-hour expiry; passing expiry is proven against the governed clock in T-219 (the browser harness uses the real clock) |
| 7 Jane authenticates | enrolment code → set-up → authenticator → own password → session; sign-in without TOTP refused |
| 8 sees INC-004 and FORENSICS | Board and FORENSICS evidence view reachable |
| 9 nothing else | INC-005, another compartment and her home page show nothing of other work |
| 10 expiry or immediate revocation | suspension confirmed; her live session is refused on the next request; sign-in refused |

## Design notes

- The People section renders only what `app.AccessService.People` decided (`CanSuspend`, `CanReset`, …). The adapter copies fields; the app-tier field is named `HoldsCustodian` so the adapter carries no policy-shaped reference (the ratchet flagged the first draft).
- *Revoke access* in the brief is suspension for a Custodian (immediate, everywhere). Ending an incident membership stays with the incident's Sponsor, as before.
- The set-up page is a sign-in variant, and the code page is a result inside Access admin. Neither adds a navigation screen.

## Review gate

This was a self red-team review of the stable branch.

| Finding | Classification | Action |
|---|---|---|
| Adding a person made two app calls from the adapter; an invalid role left a lone identity proposal pending, and the sequencing was business logic in `cmd/crab` | real defect, in scope | fixed in `e8bdfe9` (`ProposeNewPerson`), tested |
| Set-up step 2 carries the code in a hidden field | checked: signed-out pages are no-store | assertion added |
| The older *Incident members* table on Access admin lists every incident's members to any Custodian, inconsistent with U-17 | real, pre-existing, out of C2 scope | register F-9 → C3 |

## Gates

| Gate | Build environment | GitHub CI (`e8bdfe9`) |
|---|---|---|
| gofmt, vet | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| policy-reference ratchet (T-253) | unchanged baseline | pass |
| Bafflewick | 9/9 | pass |
| visual preflight | pass | pass |
