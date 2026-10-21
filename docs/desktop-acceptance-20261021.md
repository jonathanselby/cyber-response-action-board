# CRAB final acceptance evidence

- UTC run: 20261021T041239Z
- source commit: f5235059933b9c86623df93a2243d7897c3bbdf3
- version: 0.0.14-dev
- kernel: Linux 7.0.0-38-generic x86_64 GNU/Linux
- architecture: x86_64
- CPUs: 8
- memory: 30Gi total, 26Gi available
- Go: go version go1.27.2 linux/amd64
- PostgreSQL client: psql (PostgreSQL) 18.6 (Ubuntu 18.6-0ubuntu0.26.04.1)

## Source, operational and product-quality gates

### module contract — PASS

```text
	Command being timed: "bash -c go mod tidy && git diff --exit-code -- go.mod go.sum"
	User time (seconds): 0.12
	System time (seconds): 0.08
	Percent of CPU this job got: 350%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.05
	Maximum resident set size (kbytes): 22496
	Exit status: 0
```

### format contract — PASS

```text
	Command being timed: "bash -c files=$(gofmt -l ./cmd ./internal); test -z \"$files\""
	User time (seconds): 0.34
	System time (seconds): 0.05
	Percent of CPU this job got: 527%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.07
	Maximum resident set size (kbytes): 14412
	Exit status: 0
```

### vet — PASS

```text
	Command being timed: "go vet ./..."
	User time (seconds): 4.61
	System time (seconds): 1.00
	Percent of CPU this job got: 510%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.10
	Maximum resident set size (kbytes): 122492
	Exit status: 0
```

### Bafflewick 8-of-8 scenario — PASS

```text
	Command being timed: "./scripts/bafflewick-acceptance.sh"
	User time (seconds): 13.21
	System time (seconds): 3.70
	Percent of CPU this job got: 161%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:10.45
	Maximum resident set size (kbytes): 212116
	Exit status: 0
```

### visual and accessibility preflight — PASS

```text
	Command being timed: "./scripts/visual-acceptance-preflight.sh"
	User time (seconds): 2.52
	System time (seconds): 0.74
	Percent of CPU this job got: 215%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.51
	Maximum resident set size (kbytes): 156416
	Exit status: 0
```

### full tests — PASS

```text
	Command being timed: "go test -count=1 ./..."
	User time (seconds): 10.19
	System time (seconds): 1.91
	Percent of CPU this job got: 87%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:13.91
	Maximum resident set size (kbytes): 235096
	Exit status: 0
```

### native build — PASS

```text
	Command being timed: "./scripts/build.sh ./bin/crab"
	User time (seconds): 2.22
	System time (seconds): 0.47
	Percent of CPU this job got: 205%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:01.31
	Maximum resident set size (kbytes): 228376
	Exit status: 0
```

### binary provenance — PASS

```text
	Command being timed: "bash -c test \"$(./bin/crab version)\" = \"crab ${VERSION} commit=${SOURCE_COMMIT} built=${CRAB_BUILD_TIME}\""
	User time (seconds): 0.00
	System time (seconds): 0.00
	Percent of CPU this job got: 122%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.00
	Maximum resident set size (kbytes): 12716
	Exit status: 0
```

### binary startup baseline — PASS

```text
	Command being timed: "./bin/crab version"
	User time (seconds): 0.00
	System time (seconds): 0.00
	Percent of CPU this job got: 100%
	Elapsed (wall clock) time (h:mm:ss or m:ss): 0:00.00
	Maximum resident set size (kbytes): 12720
	Exit status: 0
```

## Built application

- binary: `bin/crab`
- binary bytes: 17219847
- binary SHA-256: b53220438b9a4735edd8ae9ccead899a77b4afd1acb1e5bde8f83ec3f0ee52e5
- reported version: crab 0.0.14-dev commit=f5235059933b9c86623df93a2243d7897c3bbdf3 built=2026-10-21T04:12:39Z

## Minimum-hardware observations

The per-gate blocks record measured elapsed time and maximum resident set size. The run does not replace the EDD's deliberately qualitative low-hundreds-of-MiB target with an invented threshold.

Bafflewick includes the required degraded-operation paths: optional model loss/shedding, corporate federation loss with local authentication, paper fallback/reconciliation, backup/restore/rebuild, access revocation, closure/read-only enforcement and fail-closed hostile/output controls.

## Host snapshot after gates

```text
Mem:            30Gi       4.6Gi        21Gi       511Mi       5.2Gi        25Gi
Swap:          8.0Gi          0B       8.0Gi

Filesystem      Size  Used Avail Use% Mounted on
/dev/nvme0n1p5  481G  177G  279G  39% /
```

## Result

PARTIAL — all automated gates passed, but this host is not arm64 and therefore does not close the Raspberry Pi acceptance item.
