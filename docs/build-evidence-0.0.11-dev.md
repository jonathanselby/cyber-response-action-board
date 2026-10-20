# CRAB build evidence — 0.0.11-dev

## Slice

Slice 10 — structured communications and notifier.

## Source checkpoint

The hardened Slice 10 source completed GitHub Actions run **#121** (`39383995684`) successfully before version/documentation promotion.

The source gate passed:

- module contract (`go mod tidy` leaves no diff);
- `gofmt` contract;
- `go vet ./...`;
- full `go test -race -count=1 ./...` with PostgreSQL 17;
- Linux amd64 build;
- Linux arm64 build;
- existing explicit core checkpoint acceptance gate.

The preceding hardened-source run **#120** (`39383825995`) was also fully green after exact draft-content/digest binding and webhook redirect refusal were added.

## Implemented behaviour

- Semantic communication drafts for situation reports, regulator/individual notices, executive/board updates, staff communications, holding statements and closure/PIR material.
- Issued communication is governed through the existing `Propose -> Decide` maker-checker path; immediate `Record(...)` cannot create issued communication state.
- An issue payload is exact-bound to the immutable stored draft SHA-256 and independently recomputes that digest, so changed wording cannot travel under an old approval digest.
- Situation-report cadence reuses the existing deterministic timed wait list; no third scheduler was introduced.
- Due situation reports are deterministic drafts from current authorised CRAB operating-picture state and advance the confirmed cadence on recording.
- Exercise situation reports are visibly marked.
- Notification requires confirmed issued communication plus the existing separate `notify` output permission.
- Only channels approved in the issued communication may be attempted.
- Local-file and generic webhook adapters remain thin deterministic delivery edges.
- Exercise external notification is suppressed before adapter/network I/O and the suppression is recorded immutably.
- Each configured attempt records `delivered`, `failed` or `suppressed`, including channel, destination identifier, attempt time, duration and bounded acknowledgement metadata.
- One failed channel does not silently suppress the next configured attempt.
- Webhook redirects are refused; external webhook configuration requires HTTPS; provider response bodies are not copied into outcome/error metadata.
- Material incoming enquiries are recorded and assigned without a chat/helpdesk subsystem; PostgreSQL projection and rebuild preserve assignment.
- Communication drafts, issued communications, cadence, notification outcomes and incoming enquiries rebuild from the authoritative ledger.

## Acceptance mapping exercised

Slice 10 implementation and tests cover the frozen intent of:

- AC-32 / T-45 / T-46 — current-state situation report drafting and governed issue;
- AC-33 / T-47 — every attempt recorded and later channel attempts survive an earlier failure;
- AC-34 / T-48 — exercise external I/O suppressed before the adapter call;
- AC-77 — approved version/audience/authority/channel/attempt/outcome provenance;
- AC-78 — material incoming enquiry capture and assignment;
- T-152 — governed communication cannot be issued through immediate-record semantics.

Additional attack/failure proof covers forged draft-content/hash mismatch, webhook redirect refusal, missing channel configuration and deterministic rebuild.

## Architecture retained

No message bus, chat/helpdesk platform, workflow engine, retry service, guaranteed-delivery queue, third scheduler or second database was added. Notification failure is evidence, not a reason to create hidden delivery machinery.

No release or tag is created for this development checkpoint.

The promotion/final GitHub Actions run is recorded after this document is committed.
