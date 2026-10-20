# Build evidence — 0.0.7-dev

## Checkpoint

Slice 6 — access governance, compartments and RLS decision gate.

Promoted checkpoint commit: `d5744f50081679c8b86c7a838f423b95a2b55dd7`

Documented source head verified before this evidence note: `cbb937a60ff89fc19f092673e99d4574fd608ebb`

GitHub Actions run: **#99 / `39372163491`** — success.

Passed gates:

- module contract;
- `gofmt` contract;
- `go vet ./...`;
- full `go test -race -count=1 ./...` with PostgreSQL 17;
- Linux amd64 build;
- Linux arm64 build;
- expanded focused checkpoint acceptance including Slice 6 access governance and the RLS spike.

## Functional evidence

Slice 6 adds one governed access mechanism rather than an ACL/workflow estate:

```text
Incident Sponsor proposes incident mission access
        |
        v
Custodian independently verifies/confirms
        |
        v
incident membership + GENERAL

Compartment Controller proposes restricted read-in/read-out
        |
        v
Custodian independently verifies/confirms
        |
        v
restricted compartment membership
```

The implementation proves:

- Sponsor and Compartment Controller are scoped designations, not universal roles;
- Custodian metadata authority does not grant incident-content visibility;
- access proposals use the ordinary immutable exact-version maker-checker path;
- incident membership derives GENERAL access;
- restricted compartments require explicit confirmed membership;
- effective time, expiry and revocation are preserved as access metadata and re-evaluated when the principal is reconstructed;
- an external responder can hold temporary incident and restricted-compartment access and automatically loses each scope at its configured expiry;
- administration views show access metadata/status without incident-content bodies;
- selective-release scope is limited to compartments the authenticated principal already holds;
- historical Slice 2 direct-confirmed access ledger entries remain replay-compatible.

Primary integration proof: `TestAccessGovernanceSponsorControllerCustodianAndExpiry`.

## RLS spike and decision

The frozen EDD required a real PostgreSQL row-level-security spike, not an assumption.

`TestRLSSpikePoolIsolationAndRuntimeBoundary` uses the actual constrained runtime role and pgx pool pattern against a disposable RLS table. It proves:

- missing transaction scope returns no rows;
- incident + restricted-compartment scope exposes only that incident's GENERAL and authorised restricted rows;
- wrong-scope rows remain hidden even when the SQL query itself contains no incident filter;
- transaction-local scope does not leak after commit or rollback when the same pooled connection is reused;
- runtime remains non-owner, non-superuser and without `BYPASSRLS`;
- an unscoped projection-style runtime write is also rejected.

That last result makes the production decision decisive. CRAB currently uses the same constrained runtime role for ordinary work and deterministic projection writes/rebuilds. Retaining RLS on the real projections would therefore require a second privileged projection/rebuild identity/path or a bypass mechanism, plus transaction-pinned multi-query reads. That adds a new privilege/recovery channel solely to support defence in depth.

Under EDD V0.4 section 9.11, RLS is **not retained**. Application authorisation remains primary and the empirical reason is recorded in `docs/rls-spike-decision.md`.

## Build findings

1. The first implementation failed only `gofmt`; the formatter identified two alignment-only changes and no behavioural code was weakened to make the gate pass.
2. Slice 5 had already exposed and fixed a common-operating-picture compartment-filtering risk. Slice 6 preserves those application-level negatives rather than treating RLS as a substitute for correct application authorisation.
3. The RLS spike itself worked on reads, including pooled-connection transaction-local isolation. It was rejected because the write/rebuild consequence would make CRAB more privileged and harder to recover, not because PostgreSQL RLS failed.
4. No RLS policy, bypass role, second runtime pool, per-object ACL graph, workflow engine or separate external portal was added.

## Deferred by design

Slice 6 does not add evidence/blob storage, retrieval, model assistance, notification delivery, print/paper packs, final recovery/closure flows or backup/restore completion. Those remain in the frozen later slices.

No release or tag is created for this checkpoint.
