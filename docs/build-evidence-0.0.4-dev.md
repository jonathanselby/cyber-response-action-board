# CRAB build evidence — 0.0.4-dev

## Checkpoint

- Slice: 3 — incident + semantic write kernel
- Promoted commit: `b931178f0636fbf5697402cb1a811ce1ae7a67bd`
- GitHub Actions run: `39361115727` (run #46)
- Result: success
- CI toolchain: Go 1.27.2, PostgreSQL 17

The promoted checkpoint passed the module contract, formatting, vet, the full race/PostgreSQL integration suite, Linux amd64 build, Linux arm64 build and the expanded focused acceptance set naming the Slice 3 invariants.

## What this checkpoint proves

Slice 3 now has a real application/data path rather than placeholder domain types:

1. `Record(...)` exists for observations/history and is structurally separate from controlled state.
2. `Propose(...) -> Decide(...)` exists for governed state and enforces different-human maker/checker.
3. Proposal confirmation is tied to the exact canonical payload version and the stored proposal scope.
4. Dedicated semantic schemas exist for incident declaration, fact, unknown/hypothesis, decision, action and operational role assignment; there is no generic Item CRUD domain.
5. Pending proposals are projected separately from effective state. Rejected proposals never become effective.
6. The first common operating picture is rebuilt from ledger history and keeps facts, unknowns, hypotheses, decisions, actions, operational roles and pending proposals distinct.
7. An observational fact whose text says that severity should change does not change effective severity, including after complete projection rebuild.
8. Custom operational role labels do not alter Major/Minor/Observer security roles.
9. Live/exercise mode cannot be changed during confirmation.
10. Existing-incident writes pass through the same incident/compartment authorisation boundary; Minor may record observations, Major may propose/confirm governed state, Observer remains read-only.
11. A new incident may be proposed by an eligible Major before incident membership exists, with a different Major confirming it, matching the frozen design.
12. An end-to-end test proves application service → semantic domain → authoritative ledger → PostgreSQL projection → authorised common-operating-picture read.
13. Negative PostgreSQL tests prove a controlled Action cannot be smuggled through immediate `Record` semantics and that failed mode-swapped confirmation rolls the entire ledger/projection transaction back.

## Implementation findings

The build exposed several useful issues rather than hiding them:

- Go's package namespace forced record-type constants and semantic verbs to use distinct names; the public verbs remained `Record...`, `Propose...`, `Decide` while constants became `Type...`.
- Pending proposal mode had to be projected explicitly. Inferring `live` versus `exercise` at confirmation would have created an authority gap, so migration 007 carries the original mode and confirmation checks it.
- Extending authorisation with `ActionRecord` initially replaced the file and accidentally dropped the existing Slice 2 `BindAgent` helper. `go vet` and the existing Slice 2 tests caught the regression immediately; the original agent-binding primitive was restored unchanged.
- The application service reloads the pending proposal from CRAB before confirmation. Presentation code therefore supplies identifiers and the version reviewed, not an authoritative proposal body.
- `GENERAL`/incident access semantics remain in the existing security boundary; Slice 3 did not invent a second access graph.

## Deliberately not claimed

This checkpoint is the semantic application/data kernel, not the finished browser incident experience. It does not claim:

- the Slice 4 action lifecycle, checklists or full timeline;
- obligation clocks;
- the Slice 6 RLS decision/spike;
- evidence/blob handling;
- retrieval/model/communications/print/recovery slices;
- secure operator-facing `crab setup` or browser authentication wiring.

No release or tag is created at this checkpoint.
