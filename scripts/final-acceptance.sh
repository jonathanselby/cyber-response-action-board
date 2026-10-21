#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

STAMP=$(date -u +%Y%m%dT%H%M%SZ)
BUILD_TIME=$(date -u +%Y-%m-%dT%H:%M:%SZ)
VERSION=$(tr -d '[:space:]' < VERSION)
SOURCE_COMMIT=${CRAB_SOURCE_COMMIT:-$(git rev-parse HEAD)}
OUT=${1:-"pi-acceptance-${STAMP}.md"}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT

export CRAB_SOURCE_COMMIT="$SOURCE_COMMIT"
export CRAB_BUILD_TIME="$BUILD_TIME"

if [ "$(uname -s)" != "Linux" ]; then
  echo "final acceptance runner requires Linux" >&2
  exit 1
fi

ARCH=$(uname -m)
if [ "$ARCH" != "aarch64" ] && [ "$ARCH" != "arm64" ]; then
  echo "warning: this run is not on arm64; it cannot close the Raspberry Pi acceptance item" >&2
fi

if [ -z "${CRAB_TEST_POSTGRES_ADMIN_URL:-}" ]; then
  echo "CRAB_TEST_POSTGRES_ADMIN_URL must point at the local PostgreSQL acceptance database" >&2
  exit 1
fi

TIME_CMD=""
if [ -x /usr/bin/time ]; then
  TIME_CMD=/usr/bin/time
fi

{
  echo "# CRAB final acceptance evidence"
  echo
  echo "- UTC run: ${STAMP}"
  echo "- source commit: ${SOURCE_COMMIT}"
  echo "- version: ${VERSION}"
  echo "- kernel: $(uname -srmo)"
  echo "- architecture: ${ARCH}"
  if [ -r /proc/device-tree/model ]; then
    echo "- hardware: $(tr -d '\000' </proc/device-tree/model)"
  fi
  echo "- CPUs: $(getconf _NPROCESSORS_ONLN 2>/dev/null || nproc)"
  if command -v free >/dev/null 2>&1; then
    echo "- memory: $(free -h | awk '/^Mem:/ {print $2 " total, " $7 " available"}')"
  fi
  echo "- Go: $(go version)"
  if command -v psql >/dev/null 2>&1; then
    echo "- PostgreSQL client: $(psql --version)"
  fi
  echo
  echo "## Source, operational and product-quality gates"
  echo
} >"$OUT"

run_gate() {
  local name=$1
  shift
  echo "Running: $name" >&2
  local log="$TMP/${name// /_}.log"
  local timing="$TMP/${name// /_}.time"
  if [ -n "$TIME_CMD" ]; then
    if "$TIME_CMD" -v -o "$timing" "$@" >"$log" 2>&1; then
      status=PASS
    else
      status=FAIL
    fi
  else
    start=$(date +%s)
    if "$@" >"$log" 2>&1; then
      status=PASS
    else
      status=FAIL
    fi
    end=$(date +%s)
    echo "Elapsed seconds: $((end-start))" >"$timing"
  fi
  {
    echo "### $name — $status"
    echo
    echo '```text'
    cat "$timing"
    echo '```'
    echo
  } >>"$OUT"
  if [ "$status" != PASS ]; then
    cat "$log" >&2
    echo "Acceptance failed at: $name" >&2
    exit 1
  fi
}

run_gate "module contract" bash -c 'go mod tidy && git diff --exit-code -- go.mod go.sum'
run_gate "format contract" bash -c 'files=$(gofmt -l ./cmd ./internal); test -z "$files"'
run_gate "vet" go vet ./...
run_gate "Bafflewick 8-of-8 scenario" ./scripts/bafflewick-acceptance.sh
run_gate "visual and accessibility preflight" ./scripts/visual-acceptance-preflight.sh
run_gate "full tests" go test -count=1 ./...
run_gate "native build" ./scripts/build.sh ./bin/crab
run_gate "binary provenance" bash -c 'test "$(./bin/crab version)" = "crab ${VERSION} commit=${SOURCE_COMMIT} built=${CRAB_BUILD_TIME}"'
run_gate "binary startup baseline" ./bin/crab version

{
  echo "## Built application"
  echo
  echo "- binary: \`bin/crab\`"
  echo "- binary bytes: $(stat -c %s ./bin/crab)"
  echo "- binary SHA-256: $(sha256sum ./bin/crab | awk '{print $1}')"
  echo "- reported version: $(./bin/crab version)"
  echo
  echo "## Minimum-hardware observations"
  echo
  echo "The per-gate blocks above record elapsed time and, when GNU /usr/bin/time is installed, maximum resident set size, CPU time and other process evidence. This records measured evidence rather than replacing the EDD's deliberately qualitative 'low hundreds of MiB' target with an invented threshold."
  echo
  echo "Bafflewick includes the required degraded-operation paths: optional model loss/shedding, corporate federation loss with local authentication, paper fallback/reconciliation, backup/restore/rebuild, access revocation, closure/read-only enforcement and fail-closed hostile/output controls."
  echo
  echo "## Host snapshot after gates"
  echo
  echo '```text'
  if command -v free >/dev/null 2>&1; then free -h; fi
  echo
  df -h .
  echo
  ps -eo pid,comm,rss,%cpu --sort=-rss | head -20
  echo '```'
  echo
  echo "## Result"
  echo
  if [ "$ARCH" = "aarch64" ] || [ "$ARCH" = "arm64" ]; then
    echo "PASS — source/build/test, Bafflewick and automated visual preflight completed on arm64. Confirm the hardware line above is Raspberry Pi 5 / 8 GB, review the recorded resource evidence against EDD section 25, and retain this file as AC-58 evidence. Human browser/print visual sign-off remains separate."
  else
    echo "PARTIAL — all automated gates passed, but this host is not arm64 and therefore does not close the Raspberry Pi acceptance item."
  fi
} >>"$OUT"

printf 'Acceptance evidence written to %s\n' "$OUT"
