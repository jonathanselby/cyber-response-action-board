# CRAB Slice 15 — Final Acceptance Record

**Date:** 20 October 2026  
**Baseline:** Slice 14 commit `f4c9d59084723334b2808aeeb0deaf96dce26b11`

Slice 15 is an acceptance slice. It does not add product architecture or another runtime mechanism.

## Automated acceptance

The repository CI remains the release gate. A passing Slice 15 run must pass, as one coherent run:

- module contract;
- gofmt contract;
- `go vet ./...`;
- the full unit and PostgreSQL integration suite with the race detector;
- Linux amd64 build;
- Linux arm64 build;
- core checkpoint acceptance.

The existing suite already exercises the product kernels delivered in Slices 0–14, including ledger integrity and replay, scoped authorisation, maker-checker controls, obligation clocks, evidence, retrieval/model failure behaviour, communications, print/paper generation, recovery/closure, and backup/restore.

Slice 15 adds two final presentation acceptance checks:

1. the Bafflewick operator fixture renders using only same-origin embedded assets, carries basic browser accessibility markers, and retains restrictive security/cache headers;
2. an unauthenticated browser mutation is refused before it can reach an application service.

These checks preserve the frozen acceptance intent for the offline web path rather than creating a second test framework.

Slice 15 also includes `docs/GENERATED-EDD-SLICE15.md`, produced using the existing `codebase-to-edd` method against the exact Slice 14 software commit. Its code-to-design comparison found no material contradiction with the frozen EDD v0.4.

## Acceptance evidence covered by CI

The passing repository run is the automated evidence for:

- Bafflewick software path and regression baseline;
- negative security and scope controls already represented by the existing attack/authorisation tests;
- graceful model/network dependency failure already represented by deterministic retrieval, notifier, exercise, paper and readiness tests;
- PostgreSQL projection rebuild and backup/restore tests;
- single binary builds for amd64 and arm64;
- presentation-tier dependency boundaries;
- operation without externally hosted web assets;
- browser mutation authentication boundary.

## Physical and human acceptance — deliberately not faked

Three acceptance items require evidence outside GitHub CI:

- Raspberry Pi 5 / 8 GB resource-budget run;
- human visual inspection of the live web interface;
- human visual inspection of representative A4/Letter print output.

These are release-evidence tasks, not missing product code. They must be recorded from the real rig/output. CRAB must not be labelled **fully physically accepted** until they are complete.

## Feature-complete boundary

If the Slice 15 CI run is green, the software implementation is feature-complete against the frozen PRD/EDD. The only remaining evidence is the three physical/human acceptance items above.
