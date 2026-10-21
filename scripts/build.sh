#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

OUTPUT=${1:-"./bin/crab"}
VERSION=$(tr -d '[:space:]' < VERSION)
SOURCE_COMMIT=${CRAB_SOURCE_COMMIT:-}
BUILD_TIME=${CRAB_BUILD_TIME:-$(date -u +%Y-%m-%dT%H:%M:%SZ)}

if [ -z "$VERSION" ]; then
  echo "VERSION must not be empty" >&2
  exit 1
fi

if [ -z "$SOURCE_COMMIT" ]; then
  if git rev-parse HEAD >/dev/null 2>&1; then
    SOURCE_COMMIT=$(git rev-parse HEAD)
  else
    SOURCE_COMMIT=unknown
  fi
fi

mkdir -p "$(dirname "$OUTPUT")"

go build -trimpath \
  -ldflags "-s -w -X github.com/jonathanselby/cyber-response-action-board/internal/buildinfo.Version=${VERSION} -X github.com/jonathanselby/cyber-response-action-board/internal/buildinfo.Commit=${SOURCE_COMMIT} -X github.com/jonathanselby/cyber-response-action-board/internal/buildinfo.BuiltAt=${BUILD_TIME}" \
  -o "$OUTPUT" \
  ./cmd/crab
