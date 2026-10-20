# Build evidence — 0.0.6-dev

## Checkpoint

- Slice: 5 — obligations and timed wait list
- Promoted commit: `92575a35f3b2b6677369787b5b235c538bb24ab6`
- Promotion CI: GitHub Actions run `39369888980` / run #90
- Result: success
- Governing baseline remains unchanged: PRD V0.2, EDD V0.4 FINAL, BUILD-NOTES, TEST-ACCEPTANCE-NOTES and UX-PRINT-BUILD-NOTES.

## CI proof

The promoted commit passed:

- module contract (`go mod tidy` clean);
- `gofmt` contract;
- `go vet ./...`;
- `go test -race -count=1 ./...` against PostgreSQL 17;
- Linux amd64 build;
- Linux arm64 build;
- focused checkpoint acceptance including the explicit Slice 5 obligation tests.

## Slice 5 proved

- organisation-defined obligation definitions are governed configuration rather than compiled legal rules;
- a human explicitly proposes applicability, source record/field and trigger time; CRAB does not infer legal/reportability applicability;
- elapsed-hour, calendar-day and business-day calculations are deterministic under configured timezone/calendar rules;
- invalid timezone/calendar configuration and nonexistent DST wall-time arithmetic fail visibly;
- proposed triggers create provisional clocks; confirmation makes them firm; rejection removes active effect while ledger history remains;
- amendment, extension, waiver and replacement preserve original rule/deadline history and record authority/reason;
- active overdue state is derived rather than stored independently;
- warning/deadline wake-ups use one bounded rebuildable timed wait list, not a workflow engine or delivery queue;
- exercise clocks/wake-ups are explicitly simulated;
- suggested obligation definitions remain inactive until normal maker-checker confirmation;
- untrusted trigger-time status survives pending-proposal reconstruction and confirmation;
- replacement recalculation survives PostgreSQL `timestamptz` normalisation by treating persisted trigger values as instants and applying the configured business timezone;
- active obligations enter the authorised common operating picture while waived clocks remain historical rather than active work;
- COP and obligation reads expose only `GENERAL` plus the explicitly authorised compartment.

## Implementation findings

1. The first Slice 5 pass exposed that the existing common-operating-picture application read authorised one compartment but returned the backend's complete incident picture. This was fixed at the application boundary by filtering semantic views to `GENERAL` plus the explicitly authorised compartment. The frozen RLS decision remains in Slice 6; no RLS mechanism was pulled forward.
2. PostgreSQL `timestamptz` normalises stored instants, so requiring the scanned timestamp's offset to equal the configured business timezone produced a false contradiction during replacement recalculation. Clock calculation now treats the persisted value as an instant and applies the configured timezone for calendar arithmetic.
3. Pending obligation reconstruction initially defaulted `time_status` to trusted. It now recovers the provisional clock's time-trust state so an untrusted trigger cannot silently become trusted at confirmation.
4. Checklist/action and obligation scheduling remain separate semantic concerns but share the same Garage rule: current state is rebuildable projection; the ledger is authoritative; no generic workflow runtime was introduced.

## Deliberately not claimed

This checkpoint does not claim:

- automatic legal or regulatory applicability decisions;
- external notification delivery or notifier completion;
- authenticated browser obligation UX;
- Slice 6 RLS completion;
- evidence blobs, retrieval, model assistance, print, backup/restore or later slices.

No release tag is created for this development checkpoint.
