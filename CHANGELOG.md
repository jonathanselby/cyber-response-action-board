# Changelog

## Unreleased — 0.0.6-dev

Slice 5 obligations and timed wait list.

- Added governed organisation-defined obligation configuration without embedding a legal/reportability rules engine.
- Added deterministic elapsed-hour, calendar-day and business-day calculation using configured timezone, weekend, holiday and adjustment rules.
- Added visible refusal for invalid timezone/calendar configuration and nonexistent local DST wall-time arithmetic.
- Added explicit human applicability trigger claims carrying configured rule ID, source record/field, trigger time and applicability reason.
- Added provisional clocks on proposal, firm clocks on maker-checker confirmation and removal of current effect on rejection while ledger history remains immutable.
- Added governed amendment, extension, waiver and replacement while retaining original definition/deadline and the authority source/reason for change.
- Added a bounded rebuildable timed wait list for warning/deadline wake-ups; no delivery queue, notifier or workflow engine was introduced.
- Marked exercise clocks and wake-ups simulated so exercises cannot be mistaken for real external obligations.
- Preserved untrusted trigger-time status through the pending proposal reconstruction and confirmation path.
- Added active obligation clocks to the common operating picture and kept waived clocks out of active COP work while preserving history.
- Tightened application-level common-operating-picture and obligation reads to `GENERAL` plus the explicitly authorised compartment after implementation evidence exposed a cross-compartment read risk.
- Proved suggested obligation definitions remain inactive until confirmed through the ordinary human maker-checker path.
- Proved replacement recalculation survives PostgreSQL `timestamptz` normalisation by treating stored trigger times as instants and applying the configured business timezone during calculation.
- Added deterministic rebuild proof for definitions, simulated clocks and timed waits plus bounded due-wakeup retrieval.

Authenticated browser obligation UX, external notification delivery, automatic legal applicability inference, the Slice 6 RLS decision gate and later product slices remain out of scope. No release tag should be created yet.

## 0.0.5-dev

Slice 4 actions, checklists and timeline.

- Expanded Action into the frozen operational contract: title, owner, status, priority, due time, source, notes, evidence and skip reason.
- Made overdue a derived condition rather than stored state; completed/skipped Actions do not remain overdue.
- Required a reason for skipped Actions and refused skip reasons on other Action states.
- Added governed Action lifecycle updates under the same Action ID, keeping one current projection while immutable ledger history preserves every proposal and confirmation.
- Made unresolved pending proposals immutable so a later proposal cannot replace the exact version awaiting maker-checker decision.
- Added strict data-driven checklist configuration and activation into ordinary governed Action proposals with template/step/runbook provenance, owner overrides and due offsets; no checklist service/database was introduced.
- Added explicit fact occurrence, flagging and named-human awareness times while keeping ledger recording time separate.
- Added a rebuildable incident timeline over semantic ledger history, retaining time-trust status.
- Added dedicated semantic Action and timeline renderers with owner/status/priority/due/overdue/source/checklist/evidence and distinct timeline-time labels.
- Added architecture protection proving Slice 4 does not introduce a workflow engine or Temporal-style dependency.
- Updated all Slice 3 Action fixtures to the stricter Slice 4 contract rather than weakening validation for compatibility.
- Preserved existing maker-checker, incident/compartment authorisation, live/exercise separation and rollback-on-projection-failure behaviour.

Authenticated browser routing and later product slices remain out of scope for this checkpoint. No release tag should be created yet.

## 0.0.4-dev

Slice 3 incident + semantic write kernel.

- Added typed semantic domain records for incident declaration, facts, unknowns/hypotheses, decisions, actions and operational role assignments.
- Added immediate `Record(...)` semantics for observations and `Propose(...) -> Decide(...)` semantics for governed state.
- Added SHA-256 proposal-version capture over canonical payload JSON and exact-version confirmation checks.
- Added rebuildable incident, fact, uncertainty, decision, action, operational-role and pending-proposal PostgreSQL projections.
- Added the first common operating picture with facts, unknowns, hypotheses, decisions, actions, operational roles and pending approvals kept semantically distinct.
- Kept pending proposals separate from effective state; rejection removes pending state without creating an effective row.
- Proved observational fact text cannot mutate effective severity, before or after deterministic ledger rebuild.
- Preserved custom organisation role labels without changing security roles.
- Added live/exercise proposal mode to the pending projection and reject mode-swapped confirmations.
- Added an application service that applies Major/Minor/Observer and incident/compartment authorisation before semantic writes.
- Added the frozen new-incident rule: a Major may propose a declaration before incident membership exists; a different Major confirms it.
- Confirmation reloads the pending proposal from CRAB rather than trusting caller-supplied proposal bodies.
- Added end-to-end tests through application service, domain semantics, ledger append, PostgreSQL projections and authorised common-operating-picture read.
- Added negative rollback tests proving controlled state cannot use immediate Record semantics and failed mode-swapped decisions do not enter the authoritative ledger.
- Preserved the Slice 2 agent-binding security primitive after CI caught an accidental replacement during the Slice 3 authz extension.

The semantic browser UI and later product slices remain out of scope for this checkpoint. No release tag should be created yet.

## 0.0.3-dev

Slice 2 security boundary.

- Added local password authentication, opaque hashed sessions and PostgreSQL supporting-state persistence.
- Added TOTP with AES-GCM secret wrapping, passkey verification and pinned-key RS256 OIDC verification without adding another direct runtime dependency.
- Added isolate-mode federation refusal before verifier work.
- Added exactly-two-keyholder break-glass authentication with individually revocable verification material, bounded failure limiting and immutable alert-attempt outcomes.
- Added pre-registered-route out-of-band verification that ignores caller-supplied contact details.
- Added the single deny-by-default authorisation boundary across role, incident, compartment, authoritative object scope and separate output/action permissions.
- Added rebuildable role eligibility, Custodian, incident membership, compartment membership, output-permission and isolate-mode projections.
- Made `GENERAL` derive from governed incident membership; restricted compartments require explicit confirmed read-in.
- Added opaque-ID scope re-resolution so identifier substitution cannot confer access.
- Added agent-to-named-human binding with no authority above the named human.
- Added two-human bootstrap and Custodian-recovery ceremony validation with no incident visibility or emergency read-in.
- Added explicit human signing for governed ceremony records and proved they form a valid portable ledger chain.
- Added break-glass audit and alert-edge attempt recording while preserving emergency authentication when notification fails.
- Corrected the ledger envelope to support frozen-design `system` records with no incident ID while keeping live/exercise incident IDs mandatory.
- Kept the RLS decision gate deferred to Slice 6 as designed.

Operator-facing `crab setup`/login transport is not faked in this checkpoint: secret-safe presentation/input wiring remains to be built. No release tag should be created yet.

## 0.0.2-dev

Slice 1 ledger + projection kernel.

- Added bounded `pgx` PostgreSQL pool as the first direct runtime dependency.
- Added explicit owner/runtime database-role separation and migration harness.
- Added authoritative `crab.ledger_entries` storage with exact portable entry bytes.
- Added advisory-lock ledger append with cached verified-tail check.
- Added same-transaction projection hook and rollback on projection failure.
- Added full database ledger verification and sticky integrity-failed read-only mode.
- Added verified projection rebuild and portable JSONL export commands.
- Added PostgreSQL integration tests for ownership, append/replay equality, rollback, tamper detection and restore/replay.
- Canonicalised ledger payload JSON before authoritative entry storage.

## 0.0.1-dev

First executable build checkpoint: shell, configuration, offline web surface, portable signed-ledger verification and CI baseline.
