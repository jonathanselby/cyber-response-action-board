# CRAB Bafflewick acceptance

Bafflewick is CRAB's named end-to-end operational acceptance scenario. It is not a second application, test framework or synthetic demo. The executable gate binds the existing production-path integration tests into one release contract so the complete incident story must remain green together.

## Acceptance rule

The threshold is deliberately simple: **10/10 mandatory phases must pass** (phase 10 added in 0.0.25-dev for the post-UAT remediation; phase 9 in 0.0.20-dev; the earlier threshold was 8/8). If a named proof disappears, does not execute or fails, Bafflewick fails CI. There is no partial-pass score because the PRD Definition of Done requires the whole operational scenario.

Run locally with a PostgreSQL acceptance database:

```text
CRAB_TEST_POSTGRES_ADMIN_URL='postgres://...' ./scripts/bafflewick-acceptance.sh
```

An optional first argument writes the Markdown evidence emitted by the runner to a file.

## Scenario phases

| Phase | Bafflewick outcome proved |
|---|---|
| 1 | First report and phone attachment intake, incident declaration/triage, evolving scope/severity, ordinary Actions/checklists, resource request and shift handover |
| 2 | Incident/compartment access governance, external responder expiry and explicit revocation, scoped evidence capture/download/history and projection rebuild |
| 3 | Provisional-to-firm obligation clocks, clock change history, governed situation-report cadence, notification outcomes and incoming enquiries |
| 4 | Model service loss leaves deterministic sources usable; saturation sheds model work first; corporate federation can be isolated while local authentication remains available |
| 5 | Incident Brief/paper material is derived from current state, remains readable without PostgreSQL, renders on local A4/Letter paths, and reconciled paper events preserve paper and transcription times |
| 6 | PostgreSQL-native backup/restore integrity mechanics, deterministic ledger/projection replay and standalone ledger tamper verification |
| 7 | Recovery coordination, stand-down, PIR, retention/hold, closure prerequisites, closed read-only behaviour and deterministic rebuild |
| 8 | Hostile evidence corruption/partial upload, exercise external-send suppression, managed-model sensitivity refusal and restricted-compartment print refusal all fail closed |
| 9 | With only one Major available and the host clock unverified: writes are marked TIME UNVERIFIED, a dual-controlled single-person window permits flagged self-approval of covered kinds only, related incidents never reveal a hidden incident, and the printed pack carries the organisation's emergency information |
| 10 | *The long night*: a 72-hour incident on the governed clock, run only through the browser runtime. Day and night shifts with handovers and changes of incident commander; people join (internal and external) and leave; a Major steps down to Minor while their proposal is still decided by another Major; external forensics access expires after six hours while their work remains; external legal counsel is read into LEGAL and communications is not; communications drafts a notice; eight-hour sessions end and people sign in again; counsel's account ends and stops a live session. No shared account, SQL or break-glass; the ledger verifies and every projection rebuilds identically |

## CI

`.github/workflows/bafflewick.yml` runs this acceptance gate on every push and pull request using the same PostgreSQL version and Go toolchain as the main test workflow. This is the explicit AC-31 regression gate: a regression in any mandatory Bafflewick phase fails CI.

## Deliberate boundary

The automated Bafflewick gate closes the deterministic software scenario only. It does not fabricate the two forms of release evidence that require the real target or human inspection:

- Raspberry Pi 5 / 8 GB arm64 resource measurement;
- final browser and print visual review.

Those remain separate final-release gates.
