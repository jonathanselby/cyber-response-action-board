# CRAB — Raspberry Pi final acceptance

The remaining minimum-hardware step is deliberately physical. CI proves Linux arm64 compilation; it cannot truthfully prove Raspberry Pi 5 / 8 GB resource behaviour.

## Target

- Raspberry Pi 5
- 8 GB RAM
- 64-bit Linux / `aarch64`
- local PostgreSQL suitable for the CRAB test fixture
- Go 1.27.2 toolchain
- GNU `/usr/bin/time` recommended so maximum RSS and CPU/time evidence is captured

The Pi is an engineering acceptance rig, not the production architecture target.

## One-command acceptance run

From a clean checkout of the final candidate commit, set the local PostgreSQL acceptance URL and run:

```text
export CRAB_TEST_POSTGRES_ADMIN_URL='postgres://postgres:postgres@127.0.0.1:5432/crab_test?sslmode=disable'
./scripts/final-acceptance.sh
```

The script writes `pi-acceptance-<UTC>.md` in the repository root unless another output path is supplied.

That single run executes and records:

- module contract;
- gofmt contract;
- vet;
- Bafflewick 9/9 operational scenario, including degraded-operation/failure paths;
- automated browser/print accessibility/visual preflight;
- full Go/PostgreSQL tests;
- native arm64 build;
- CRAB binary startup baseline;
- binary size/hash/version;
- per-gate elapsed/resource evidence where GNU `time` is available;
- hardware, CPU, memory, filesystem and post-run process snapshot.

## Acceptance decision

Do not convert the EDD's qualitative resource target into an invented precise threshold after the fact. Review the measured evidence against EDD section 25:

- CRAB process is intended to remain in the low hundreds of MiB under normal operation;
- queues and hostile uploads remain bounded;
- model memory is not required core capability;
- ordinary local operation should feel immediate;
- core work continues when optional model work is shed.

The executable tests prove the bounded/shedding/degraded-operation behaviours. The Pi evidence file records actual target-host resource observations.

If a budget looks materially wrong, keep the evidence file, reopen only that failed behaviour, fix it, and rerun. Do not reopen the architecture wholesale.

## What still cannot be automated

After a passing Pi run, the only remaining release gate is human rendered visual review of the representative browser and printed outputs listed in `docs/visual-acceptance.md`.
