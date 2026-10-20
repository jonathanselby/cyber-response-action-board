# Changelog

## Unreleased — 0.0.10-dev

Slice 9 model assistance.

- Added five small static, prompt-versioned CRAB draft specifications for retrieval summary, situation report, board update, notification language and PIR summary.
- Reused the Garage three-lane harness contract with explicit `simple`, `standard` and `complex` lanes selected by the caller; no hidden classifier or automatic model selection was introduced.
- Added local Ollama plus managed Azure OpenAI/Foundry, Amazon Bedrock and Google Vertex provider adapters using standard-library HTTP only.
- Kept deterministic authorised retrieval ahead of model generation so only the already-authorised citation set can enter a prompt.
- Added a managed-provider sensitivity policy gate and the existing separate `model_send` output permission before any managed-provider I/O.
- Added prompt/spec/provider/model/source provenance, prompt SHA-256, duration and provider-reported token traces without routinely logging prompt bodies.
- Marked all model output explicitly untrusted. Deterministic `validated` output means structural validation only and is not human approval or effective governed state.
- Added visible failure behaviour for unavailable providers and invalid output while retaining authorised source provenance and any returned untrusted text.
- Added a hard no-reroute rule: a failed configured lane/provider is not silently replaced by another provider.
- Added bounded, non-blocking model capacity so new model work is shed first while deterministic retrieval and core incident work remain available.
- Added local HTTP wire-contract tests for all four provider adapters and prevented managed-provider error strings from echoing provider response bodies.
- Added live/exercise mode explicitly to the managed-model policy boundary.
- Added Slice 9 model/security/provider tests to the explicit core checkpoint gate and recorded build evidence in `docs/build-evidence-0.0.10-dev.md`.

Live provider credentials/configuration and authenticated operator model routes are not faked at this checkpoint. No release tag should be created yet.

## Unreleased — 0.0.9-dev

Slice 8 governed corpus and deterministic retrieval.

- Added governed Markdown/DOCX corpus admission using the existing maker-checker ledger semantics.
- Reused immutable PostgreSQL blob storage for exact original source bytes; no filesystem, object store or second persistence engine was introduced.
- Added a hard 16 MiB corpus source limit enforced while streaming, independent of the larger configurable evidence limit.
- Added bounded passive DOCX ZIP/XML handling with entry, aggregate-expanded-content and document-XML limits; only `word/document.xml` text is indexed and embedded macro/active content is never executed.
- Added deterministic Markdown line-range and DOCX paragraph locators with rebuildable lexical terms and source provenance.
- Added database incident/compartment filtering before candidate passages reach the lexical ranker, including organisation-wide governed sources.
- Added deterministic lexical ranking, stable tie-breaks and citations bound to source ID, immutable source SHA-256 and source locator.
- Added explicit unsupported-query refusal when no authorised passage matches instead of fabricating support.
- Proved retrieval operates without embeddings, a vector database, local LLM or network connection.
- Added negative tests where a restricted-compartment passage and another incident's passage are deliberately stronger lexical matches; neither enters the authorised ranking set.
- Proved authorised restricted-compartment retrieval still works after read-in and that corpus projection rebuild preserves citation identity while leaving immutable source bytes untouched.
- Closed the frozen incident-declaration invariant: confirmation now derives the declaration maker's initial Sponsor, incident membership and `GENERAL` membership from the declaration ledger record, including combined rebuild proof.
- Added Slice 8 retrieval/security tests to the explicit core checkpoint gate.

Model drafting remains Slice 9. Authenticated browser ingestion/search routes are not faked at this checkpoint. No release tag should be created yet.

## 0.0.8-dev

Slice 7 hostile evidence and immutable blobs.

- Added bounded streaming of untrusted evidence bytes into ordinary PostgreSQL metadata/chunk rows without introducing an object store or media service.
- Added SHA-256 calculation while streaming, opaque random per-upload blob IDs and exact-byte retrieval.
- Explicitly disabled content-addressed/cross-incident deduplication: identical uploads in different incidents receive different blob IDs.
- Added staged transaction semantics so partial read failures and configured-size-limit failures leave no committed blob/chunks, evidence projection or ledger entry.
- Restricted runtime blob privileges to `SELECT, INSERT`; runtime cannot update/delete committed blob metadata or chunks.
- Added immutable evidence records that exactly bind blob hash/size/filename/media metadata to incident, mode and compartment before the blob is usable as evidence.
- Added scoped evidence register plus handling history/current-handler projection and deterministic projection rebuild without rewriting durable blob bytes.
- Added current authorisation on evidence detail/register/history and authoritative-scope re-resolution on attachment access; guessed evidence IDs outside scope are denied.
- Kept attachment download as a separate output permission from ordinary read.
- Added integrity verification before attachment bytes are emitted, including corruption detection after privileged database tampering.
- Added safe-inline media policy and response headers: passive allowlisted image/audio/video may render inline only when detected type is safe and declared/detected types agree; HTML, SVG, PDF, active/ambiguous types and mismatches fall back to `application/octet-stream` attachment download with `nosniff` and `no-store`.
- Added filename sanitisation for attachment response headers.
- Preserved collector/source/time/location/current-handler/related-reference metadata without claiming forensic acquisition, malware analysis or certified chain of custody.
- Kept the evidence capture service reader-based so phone/browser streams do not need whole-file buffering; secure authenticated browser route composition remains later presentation work rather than an unauthenticated shortcut.
- Added the Slice 7 evidence/security tests to the explicit core checkpoint gate and retained the safe-inline spike decision in `docs/safe-inline-media-spike-decision.md`.

No release tag should be created yet.

## 0.0.7-dev

Slice 6 access governance and compartments.

- Added governed Incident Sponsor and Compartment Controller designations without creating new universal application roles.
- Added Sponsor-proposed incident membership and Controller-proposed restricted-compartment read-in/read-out, independently confirmed by a Custodian through the normal maker-checker path.
- Kept Custodian authority metadata-only: administration views expose identity/access scopes, designations, grant windows and status without incident-content bodies.
- Added effective-time, expiry and revocation to incident and compartment membership projections; query-time principal reconstruction removes expired access without deleting ledger history.
- Kept `GENERAL` derived from active incident membership and required explicit restricted-compartment membership.
- Added immutable exact-version pending access proposals and revalidation of mission authority at decision time.
- Added external-responder expiry coverage and selective-release scope limited to compartments the current principal actually holds.
- Ran the frozen PostgreSQL RLS spike using the real non-owner runtime role and pgx pooled-connection pattern.
- Proved transaction-local RLS scope blocks broad/wrong-scope reads and clears across both commit and rollback on a reused pooled connection.
- Proved the same RLS policy blocks unscoped projection-style runtime writes; retaining RLS would therefore require a second privileged projection/rebuild role/path or bypass mechanism plus transaction-pinned repository reads.
- Rejected production RLS for V0.4 under the frozen EDD gate because that additional privilege/recovery complexity outweighs the defence-in-depth gain; the empirical test and decision record remain in source.
- Preserved the existing runtime-role minimums: not table owner, not superuser and not `BYPASSRLS`.
- Preserved the existing application-level cross-incident, cross-compartment and identifier-substitution negative controls.

Authenticated browser access-administration UX and later product slices remain out of scope for this checkpoint. No release tag should be created yet.

## 0.0.6-dev

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

Authenticated browser obligation UX, external notification delivery, automatic legal applicability inference and later product slices remain out of scope. No release tag should be created yet.

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
