# CRAB build evidence — 0.0.34-dev

**Date:** 2 November 2026
**Status:** release candidate for Phase 4 human UAT. Post-UAT remediation, chunk C13 (final automated gate and UAT pack). **The architecture is frozen**: from here, only UAT fixes are made, each at the smallest size.
**Governing documents:** `docs/remediation-build-plan.md` C13, §7–§8; `docs/uat-remediation-triage.md` S28; `TEST-ACCEPTANCE-NOTES.md` T-247
**Implementation commit:** `08da499`

## Scope

| Item | Result | Evidence |
|---|---|---|
| One final-gate script implementing brief §26 | done: `scripts/final-gate.sh` | T-247 |
| Every register row has a disposition | done: 37 finding rows, none open | the gate's *register fully dispositioned* check |
| Phase 3 outcome recorded (F-6) | done: NOT ACCEPTED, with the post-UAT brief as its outcome; rows not scored individually, so none is claimed | `docs/visual-acceptance.md` |
| Phase 4 UAT script | done: fresh install, two browser sessions, 11 scored sections, about 90 minutes | `docs/uat-phase4-script.md` |
| Disk-full and other database refusals worded for people (noted in C10) | done | `TestDatabaseRefusalsAreWordedForPeople` |

## What the final gate runs

Each gate is reported PASS, FAIL or NOT RUN. A skipped or missing gate is never reported as PASS, and the gate passes only when every gate ran and passed.

1. `gofmt`
2. `go vet` of the release build and of the proof build
3. a release binary containing no failpoints
4. every register row dispositioned
5. `go test -race` with PostgreSQL. This covers:
   - the policy ratchet at zero;
   - import boundaries;
   - the host-clock allowlist;
   - the access abuse suite;
   - every end-to-end journey, including notifications.
6. Bafflewick, 10 phases
7. proof harnesses:
   - destructive kill points;
   - database outage and full disk;
   - populated backup and restore;
   - upgrade from the baseline release to schema 22;
   - load (50 users);
   - accelerated 72-hour soak.
8. visual preflight
9. browser journeys and axe on Chromium, Firefox and WebKit, with phone emulation

## Results

**GitHub CI, `final-gate` workflow on `08da499`:** every one of the 10 gates PASS. *PASS: every automated gate ran and passed.*

The per-concern workflows on the same commit also passed: test, proof, bafflewick, uat-browser, visual-preflight.

**Build environment** (a partial run, `FINAL_GATE_ALLOW_NOT_RUN=1`):

- every gate that ran passed;
- the database-and-storage proof was skipped, because there is no PostgreSQL service control here;
- browsers ran on Chromium only, without axe.

## Not claimed

- **Human acceptance.** Phase 4 UAT is pending (`docs/uat-phase4-script.md`).
- **Visual sign-off.** AC-43 remains open until the Phase 4 rows pass.
- **Raspberry Pi measurements.** This run is handed over as `scripts/final-acceptance.sh`.
- **Header logo and signed-out navigation.** These follow go-app-shell v0.1.0 (U-14).

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| A partial gate run could have looked like a pass | design | NOT RUN is reported, skips are listed, and the gate exits 2 unless partial runs are explicitly allowed |
| The upgrade proof expected schema 21 | real (caught in C12) | it now reads the schema version from the code |

## Next

1. **Phase 4 human UAT.**
2. **Fix findings.** Fix-now findings go through the chunk loop.
3. **Close the build** (`docs/remediation-build-plan.md` §8):
   - promote the version;
   - refresh the release evidence;
   - run the final CI;
   - tag and release with one-sentence notes;
   - stop.
