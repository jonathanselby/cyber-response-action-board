# CRAB build evidence — 0.0.31-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C10 (failure and recovery proofs, time behaviour, F-4, F-11).
**Governing documents:** `docs/uat-remediation-triage.md` §3.10, §3.13, §6; `docs/remediation-build-plan.md` C10; `TEST-ACCEPTANCE-NOTES.md` §5 (T-240–T-244)
**Implementation commits:** `532c1c3`, review fix `5224b02` (harness only)

## The rule

CRAB may become slower; it must not become wrong. Every proof ends with the same oracle:

- the ledger verifies;
- projections are fingerprinted, rebuilt and fingerprinted again, and must be identical;
- every stored blob re-hashes to its recorded SHA-256;
- every 2xx write is exactly one record.

## Scope

| Item | Result | Tests |
|---|---|---|
| `internal/proof`: failpoints compiled in only with `-tags proof`. A release build has no live failpoint, and its binary does not contain the trigger string (checked in CI) | done | `TestReleaseBuildHasNoFailpoints`, proof workflow step |
| Destructive: SIGKILL of `crab serve` at blob-stored, ledger-before-commit and ledger-after-commit, and of `crab backup` part-way | done; committed once or not at all at every point | T-240 `TestProofDestructive` |
| F-4: orphan blobs listed by `crab readiness` and on the Readiness page, never deleted | done | T-240 |
| Database: PostgreSQL stopped and started under a running CRAB; disk full on a 24 MB tablespace holding the ledger | done | T-241 `TestProofDatabaseAndStorage` |
| Populated backup restored onto a replacement database: 60 tables and every blob hash identical | done | T-242 `TestProofBackupRestorePopulated` |
| Upgrade from 66b188a (schema 17), including a killed migration, a failed migration and the real one | done | T-243 `TestProofUpgradeFromBaseline` |
| R-19: backwards-clock guard in the kernel; resume and UK daylight-saving cases | done | T-244 |
| F-11: the remaining host-clock reads moved to the governed clock (incident reads, CLI, access pages); a static check allows six reasoned exceptions | done | `TestHostClockReadsAreConfined` |
| `.github/workflows/proof.yml`: runner PostgreSQL 16 (so it can be stopped), a tmpfs for disk-full, full history for the baseline build | done | — |

## Defects the harnesses demonstrated (fixed)

| Harness | Defect | Fix |
|---|---|---|
| Database (T-241) | A database outage **signed everyone out**: any error loading the frame cleared the session cookie, so once PostgreSQL returned every browser had to sign in again (seen as 401s) | A backend error now answers 503 *nothing was saved* with the typed input, and keeps the session. `TestDatabaseOutageKeepsTheSession` |
| Upgrade (T-243) | **No way to migrate a deployed database.** No command ran migrations, so a new release could only refuse an old schema | `crab migrate`: owner connection, one transaction, runtime role granted, ledger verified after. The refusal says to run it |
| Upgrade (T-243) | The migration-021 decision-loop backfill **rounded** ledger nanoseconds to microseconds where the driver **truncates**, so a rebuild after upgrade differed | the backfill truncates; caught only with real records from the baseline release |

## Not covered, stated plainly

- The **notification-before-attempt** failpoint exists, but no journey drives it yet: issuing a communication needs a configured channel. The general ledger kill points cover the record it writes.
- A **clock that is synchronised but wrong** cannot be detected without an external reference, and none is added (triage §3.13). The backwards case is caught.
- **Disk full on the export directory** was already proven (last good copy kept) and was not repeated.

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| In CI, `systemctl stop postgresql` returned before the cluster stopped, so a write succeeded against a live database and the outage case was vacuous | real harness defect (found by CI) | fixed: `pg_ctlcluster --mode fast` stops synchronously, and the harness fails if PostgreSQL still answers |
| A disk-full refusal shows PostgreSQL's error text, including a file path, to the person | presentation; the failure is visible as required | recorded for C13 wording review |
| The orphan check could list a blob mid-upload | warn only; such blobs gain their record moments later | recorded |

## Gates

| Gate | Build environment | GitHub CI |
|---|---|---|
| gofmt, vet (with and without `-tags proof`) | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
| uat-browser | Chromium: 10 cases, 0 failing | pass |
| proof (new) | 4 of 4 pass | pass |
