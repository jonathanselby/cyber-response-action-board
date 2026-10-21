# CRAB build evidence — 0.0.14-dev

**Date:** 21 October 2026  
**Status:** development checkpoint — code complete, physical Pi acceptance still to be run

## Scope closed in this checkpoint

This checkpoint completes the recovery work after the failed Slice 13/14 attempt without reopening the frozen PRD V0.2 or EDD V0.4.

Implemented since the clean Slice 12 restore:

- secure operator runtime and browser authentication path;
- two-person local bootstrap through the operator UI;
- authenticated incident declaration, fact recording, governed Action proposal and exact-version decision paths;
- local JSON API on the same operator runtime;
- authenticated MCP stdio adapter on the same operator runtime and authorisation path;
- authenticated Incident Brief and Emergency Response Pack CLI generation using a private operator-session file;
- non-loopback deployment refusal unless TLS certificate/key and an HTTPS public base URL are configured;
- PostgreSQL-native custom-format backup with SHA-256 metadata and atomic file commit;
- restore through a maintenance database connection, full ledger verification, deterministic projection rebuild and restore-rehearsal evidence;
- readiness derived from actual key, backup/restore, export-path, storage and paper-pack evidence;
- local Custodian recovery command requiring a private request file, unavailable while a normal Custodian quorum is demonstrably usable;
- file-first deployment composition for optional model lanes and notification channels;
- local model lanes constrained to loopback endpoints so they cannot be used as an undeclared managed-provider bypass;
- managed model endpoints constrained to HTTPS;
- managed-provider authentication referenced through deployment environment names rather than stored in repository configuration;
- notification channel external/local status derived from channel type rather than caller-controlled configuration;
- private-permission enforcement for deployment configuration files.

## Recovery lineage

The failed experimental Slice 13/14 path was not extended.

The repository was restored to the known-clean Slice 12 checkpoint and rebuilt forward. The recovered implementation is 12 focused commits ahead of that restore before this final deployment-composition checkpoint.

## CI evidence

The deployment-composition head `a6bc366dbe8a522d027388c3d97bfdae587a37fc` completed GitHub Actions workflow run `39521322598` successfully.

The run passed:

- module contract (`go mod tidy` leaves `go.mod` / `go.sum` unchanged);
- gofmt contract;
- `go vet ./...`;
- full unit and PostgreSQL integration suite with the race detector;
- Linux amd64 build;
- Linux arm64 build;
- explicit core checkpoint acceptance suite.

The full race-enabled suite includes the deployment-composition tests added in this checkpoint.

## Security result

No recovery shortcut was introduced.

In particular:

- browser/API/MCP/print paths reuse authenticated principals and the existing application authorisation boundary;
- print remains a separately authorised output action;
- Custodian recovery restores Custodian administrative authority only and does not grant incident or compartment access;
- break-glass and recovery material are never accepted through ordinary command arguments;
- non-loopback HTTP deployment remains refused;
- external notification status is not operator-declared; webhook channels are always treated as external and therefore remain exercise-suppressible;
- managed model lanes remain separately subject to sensitivity policy and `model_send` authorisation in the model service;
- model deployment configuration cannot relabel a remote Ollama endpoint as a local lane.

## Acceptance still requiring physical evidence

The application code and CI gates are complete for this development checkpoint.

The remaining release evidence is deliberately not fabricated in CI:

1. run the Bafflewick operational scenario against the composed application;
2. run the Raspberry Pi 5 / 8 GB arm64 budget fixture and record measured results;
3. perform final browser and print visual review on rendered output;
4. compare a generated codebase-to-EDD description with frozen EDD V0.4 for material architectural drift.

Those are acceptance activities, not missing product mechanisms. A failed acceptance result reopens only the failed behaviour; it does not justify another architectural rewrite.

## Release rule

Do not create a stable release tag from this checkpoint until the physical/visual acceptance evidence above is recorded.
