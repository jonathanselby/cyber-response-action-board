# CRAB build evidence — 0.0.5-dev

## Checkpoint

**Slice 4 — actions, checklists and timeline**

Promoted commit: `9bdf28d3cb2fb16e6b98170431bf7da33f4d8a53`

GitHub Actions run: **#70 — `39364859381`**

Result: **green**.

Passed gates:

- module contract (`go mod tidy` produces no diff);
- `gofmt` contract;
- `go vet ./...`;
- full unit + PostgreSQL integration suite under the race detector;
- Linux amd64 build;
- Linux arm64 build;
- focused checkpoint acceptance including the explicit Slice 4 contracts.

## What this checkpoint proves

### Actions

Action is still one governed CRAB semantic object, not a workflow subsystem. The effective Action carries owner, status, priority, optional due time, source, notes, evidence, optional checklist provenance and skip reason.

A later Action change uses the same Action ID through `Propose(...) -> Decide(...)`. PostgreSQL keeps one rebuildable current row while the immutable ledger retains every proposal and confirmation.

Overdue is derived from due time plus current time. It is not stored as another source of truth. Completed and skipped Actions do not remain overdue.

Skipped Actions require a reason. Missing owner, status, priority or source is rejected.

### Pending proposal integrity

An unresolved pending proposal is immutable. A later version using the same governed object ID cannot replace the exact payload awaiting maker-checker decision. The failed append rolls back, including the attempted ledger entry.

### Checklists

Checklist templates are strict validated JSON configuration rather than another database/service. Activating a template expands its steps into ordinary Action proposals, preserving runbook source, template ID and step ID while applying owner overrides and due offsets.

No checklist engine or workflow engine was introduced.

### Timeline

The incident timeline is a rebuildable projection of semantic ledger history. For observations it preserves these concepts separately:

- occurrence time;
- flagged time;
- ledger recording time;
- named-human awareness time;
- ledger time-trust status.

The integration suite proves those values survive projection rebuild without collapsing into one timestamp.

### Presentation

Slice 4 adds dedicated semantic renderers for Actions and timeline entries. They expose operator concepts directly rather than generic Item/CRUD language. They are not exposed through an improvised unauthenticated incident route; authenticated browser wiring remains later presentation work.

## Useful build findings

CI materially improved this slice:

1. Tightening Action to the frozen owner/status/priority/source contract broke old Slice 3 test fixtures. The fixtures were corrected rather than weakening validation.
2. Vet caught an incorrect `ledger.Entry.Sequence` reference; the actual field is `Seq`.
3. New integration-test setup mistakes were caught before behavioural execution and corrected without production changes.
4. Reviewing the initially green implementation exposed a semantic risk: `ON CONFLICT DO UPDATE` could replace an unresolved pending proposal. A database invariant now rejects updates to pending proposal rows until confirmation/rejection removes them.
5. Checklist-template persistence was removed after simplification review. Validated configuration plus ordinary Action proposals satisfies the product requirement with less machinery.

## Architecture result

Slice 4 adds no workflow engine, queue, message bus, alternate store or new micro-component. It extends the existing domain/application/PostgreSQL projection path only.

No release or tag is created at this checkpoint.
