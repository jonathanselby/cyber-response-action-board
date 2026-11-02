# CRAB build evidence — 0.0.32-dev

**Date:** 2 November 2026
**Status:** development checkpoint. Post-UAT remediation, chunk C11 (load and soak).
**Governing documents:** `docs/uat-remediation-triage.md` §3.10, §6; `docs/remediation-build-plan.md` C11; `TEST-ACCEPTANCE-NOTES.md` §5 (T-238, T-239)
**Implementation commit:** `068065a`

## Scope

| Item | Result | Tests |
|---|---|---|
| Load driver (`-tags proof`): 50 responders with their own principals and scopes; five incidents each with board-stress volumes (41 actions, 18 decisions, 13 unknowns, 8 waiting approvals); real `crab serve`; correctness oracle, scope-marker leak check, one record per accepted write | done | T-238 `TestProofLoad` |
| Accelerated soak: 72 hours on the governed clock while responders work and sign in again as sessions end; heap, goroutines, sessions and connections sampled 30 times | done | T-239 `TestProofSoak` |
| Resource sampling from inside the server (goroutines, heap) and a CPU profile, both proof-only; the release build has neither | done | proof workflow release check |
| Fix measured bottlenecks only, with before and after numbers | one fixed (below); F-5 measured and left | — |
| Pi run handed over as one command: `scripts/final-acceptance.sh` runs load and soak and writes the reports into its single evidence file | done | — |
| `soak` workflow: full profile (10 minutes of load, then 6 real hours covering 72), weekly and on demand | done | — |

## Measured bottleneck and fix

A CPU profile of the load run (build host, 2 CPUs, short profile) showed:

- `Frame` took **27.7%** of CPU. It runs on every page and built the whole Approvals view, self-approvals included, just to count waiting items.
- The approvals queue read each incident's full operating picture, timeline included, to get a title and its pending list: **16%** of CPU was spent in `readTimeline` alone.

The fix:

- `Frame` asks only for the count.
- New light readers: `IncidentHeader`, `PendingProposals` and `SelfApprovedSince`.
- The self-approval listing moved into the application tier, with its visibility rule.

| Short profile, same host | Before | After |
|---|---|---|
| Requests in 60 s | 6,144 | 10,605 (+73%) |
| Approvals p50 / p95 | 457 / 565 ms | 175 / 237 ms |
| Command Summary p50 / p95 | 577 / 717 ms | 268 / 353 ms |
| Board p50 / p95 | 493 / 609 ms | 334 / 431 ms |
| `Frame` share of CPU | 27.7% | 16.7% |

**F-5** (configuration read on every request): `config.Load` was **0.04%** of CPU under the same load. It is not a bottleneck, so it was not changed.

## Results, GitHub CI (4 CPUs, short profile)

**Load:** 50 responders for 60 s.

- 13,322 requests (222 per second): 12,272 × 200, 1,050 × 303 accepted writes.
- No 5xx and no failed requests.
- No scope leaks.
- Every accepted write is exactly one record; the oracle passed.

| Operation | p50 | p95 | p99 |
|---|---|---|---|
| Approvals | 140 ms | 165 ms | 176 ms |
| Board | 270 ms | 310 ms | 324 ms |
| Command Summary | 219 ms | 258 ms | 272 ms |
| Evidence | 218 ms | 256 ms | 268 ms |
| Record a fact | 70 ms | 91 ms | 100 ms |

The server process peaked at 39 MiB resident, 108 goroutines, a 12 MiB heap and 5 PostgreSQL connections (the pool cap is 8).

**Soak:** 72 simulated hours in 3 real minutes. Sessions expired and were renewed about 9 times per responder.

| | Sample 1 | Sample 10 | Sample 20 | Sample 30 |
|---|---|---|---|---|
| Heap MiB | 3.3 | 3.4 | 3.6 | 3.6 |
| Goroutines | 26 | 26 | 26 | 21 |
| Sessions | 22 | 10 | 10 | 10 |
| PostgreSQL connections | 4 | 4 | 4 | 4 |

There is no growth after warm-up, and expired sessions are swept (F-3). Of 23,859 requests, 5 reads returned 403. That happens when a session expired between the frame check and the page load, because the clock advances 24 minutes every real second. In real time that window is microseconds. No write was lost or doubled.

## Not covered, stated plainly

- **No Raspberry Pi run.** No Pi numbers are claimed. The run is handed over as `scripts/final-acceptance.sh`, which now includes load and soak.
- **Full profile not yet run.** The 6-hour soak runs in the weekly `soak` workflow; its first run is after this checkpoint.
- **No timers to fire.** "Timed waits fire exactly once" is met by design rather than measured: deadlines are judged when read (T-244), so there is nothing to fire twice.

## Review gate

This was a self red-team review.

| Finding | Classification | Action |
|---|---|---|
| Moving the visibility check for self-approvals into the adapter would have put a policy rule in the composition tier | real; caught before commit | moved into `app.ApprovalsService.RecentSelfApprovals` |
| The visual stress fixture writes projection-only rows, so the oracle's rebuild check fails on it | harness | the load fixture builds the same volumes through the ledger |
| Soak 403s on reads at a session boundary | an artefact of the accelerated clock | recorded above |

## Gates

| Gate | Build environment | GitHub CI (`068065a`) |
|---|---|---|
| gofmt, vet (with and without `-tags proof`) | clean | pass |
| `go test -race -count=1 ./...` | pass | pass |
| Bafflewick | **10/10** | pass |
| visual preflight | pass | pass |
| uat-browser | — | pass |
| proof (destructive, database, backup, upgrade, load, soak) | pass | pass |
