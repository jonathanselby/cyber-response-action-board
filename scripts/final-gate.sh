#!/usr/bin/env bash
# CRAB final automated gate (brief §26, T-247): every automated gate in one
# run, with one report.
#
#   scripts/final-gate.sh [report.md]
#
# Gates: format; vet (release and proof builds); race tests with PostgreSQL
# (architecture ratchet, import boundaries, host-clock allowlist, abuse
# suite, end-to-end journeys); Bafflewick; visual preflight; proof harnesses
# (destructive, database and storage, backup and restore, upgrade, load,
# soak); browser journeys and axe; release build free of failpoints; every
# remediation register row dispositioned.
#
# A gate whose prerequisites are missing is reported NOT RUN, never PASS.
# The gate passes only when every gate ran and passed, unless
# FINAL_GATE_ALLOW_NOT_RUN=1 (a partial run, for a developer machine).
#
# Needs: CRAB_TEST_POSTGRES_ADMIN_URL. For the database-and-storage proof:
# CRAB_PROOF_PG_STOP, CRAB_PROOF_PG_START, CRAB_PROOF_FULL_DIR. For browsers:
# uat/node_modules installed (npm ci) and Playwright browsers.
set -uo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"
STAMP=$(date -u +%Y%m%dT%H%M%SZ)
OUT=${1:-"final-gate-${STAMP}.md"}
LOGS=$(mktemp -d)
trap 'rm -rf "$LOGS"' EXIT
failed=0
notrun=0

{
	echo "# CRAB final automated gate"
	echo
	echo "- UTC run: ${STAMP}"
	echo "- version: $(tr -d '[:space:]' <VERSION)"
	echo "- source commit: $(git rev-parse HEAD 2>/dev/null || echo unknown)$(git diff --quiet 2>/dev/null || echo ' (uncommitted changes)')"
	echo "- Go: $(go version)"
	echo "- host: $(uname -srm)"
	echo
	echo "| Gate | Result | Note |"
	echo "|---|---|---|"
} >"$OUT"

record() { echo "| $1 | **$2** | $3 |" >>"$OUT"; }

# gate NAME COMMAND... runs a gate and records PASS or FAIL with its log tail.
gate() {
	local name=$1
	shift
	local log="$LOGS/${name//[^a-zA-Z0-9]/_}.log"
	echo "== $name" >&2
	if "$@" >"$log" 2>&1; then
		local skipped
		skipped=$(grep -c -- '--- SKIP' "$log" || true)
		if [ "$skipped" != "0" ]; then
			record "$name" "PASS" "$skipped test(s) skipped: $(grep -- '--- SKIP' "$log" | sed 's/.*--- SKIP: //; s/ (.*//' | tr '\n' ' ')"
		else
			record "$name" "PASS" ""
		fi
	else
		failed=1
		record "$name" "FAIL" "see log below"
		{
			echo
			echo "<details><summary>$name log (last 40 lines)</summary>"
			echo
			echo '```text'
			tail -40 "$log"
			echo '```'
			echo "</details>"
		} >>"$LOGS/failures.md"
	fi
}

not_run() {
	notrun=1
	record "$1" "NOT RUN" "$2"
}

gofmt_clean() {
	local files
	files=$(gofmt -l .)
	[ -z "$files" ] || { echo "$files"; return 1; }
}

release_free_of_failpoints() {
	local bin="$LOGS/crab"
	go build -o "$bin" ./cmd/crab && ! grep -aq CRAB_FAILPOINT "$bin"
}

register_dispositioned() {
	# Every finding row (R-, F-) has a result, and none is open.
	local bad
	bad=$(grep -E '^\| (R|F)-[0-9]+ ' docs/remediation-register.md | awk -F'|' '{r=$(NF-1); gsub(/^ +| +$/, "", r); if (r == "" || tolower(r) ~ /^(open|pending|blocked)/) print $2 ": " r}')
	[ -z "$bad" ] || { echo "undispositioned rows:"; echo "$bad"; return 1; }
	echo "$(grep -cE '^\| (R|F)-[0-9]+ ' docs/remediation-register.md) rows dispositioned"
}

gate "gofmt" gofmt_clean
gate "go vet (release)" go vet ./...
gate "go vet (proof build)" go vet -tags proof ./...
gate "release build has no failpoints" release_free_of_failpoints
gate "register fully dispositioned" register_dispositioned

if [ -z "${CRAB_TEST_POSTGRES_ADMIN_URL:-}" ]; then
	not_run "race tests with PostgreSQL" "CRAB_TEST_POSTGRES_ADMIN_URL not set"
	not_run "Bafflewick" "CRAB_TEST_POSTGRES_ADMIN_URL not set"
	not_run "proof harnesses" "CRAB_TEST_POSTGRES_ADMIN_URL not set"
else
	gate "race tests with PostgreSQL (ratchet, boundaries, clock allowlist, abuse suite, journeys)" go test -race -count=1 -timeout 30m ./...
	gate "Bafflewick" ./scripts/bafflewick-acceptance.sh
	if [ -z "${CRAB_PROOF_PG_STOP:-}" ] || [ -z "${CRAB_PROOF_PG_START:-}" ] || [ -z "${CRAB_PROOF_FULL_DIR:-}" ]; then
		notrun=1
		gate "proof harnesses (database-and-storage skipped: CRAB_PROOF_PG_* not set)" env CRAB_PROOF_PROFILE="${CRAB_PROOF_PROFILE:-short}" go test -tags proof -count=1 -v -timeout 40m -run '^TestProof' ./cmd/crab
	else
		gate "proof harnesses" env CRAB_PROOF_PROFILE="${CRAB_PROOF_PROFILE:-short}" go test -tags proof -count=1 -v -timeout 40m -run '^TestProof' ./cmd/crab
	fi
fi
gate "visual preflight" ./scripts/visual-acceptance-preflight.sh

if [ -d uat/node_modules/playwright ] && command -v node >/dev/null 2>&1; then
	gate "browser journeys and axe" bash -c 'cd uat && node run.mjs'
else
	not_run "browser journeys and axe" "uat/node_modules not installed (cd uat && npm ci && npx playwright install --with-deps)"
fi

{
	echo
	if [ -f "$LOGS/failures.md" ]; then
		echo "## Failures"
		cat "$LOGS/failures.md"
		echo
	fi
	echo "## Result"
	echo
	if [ "$failed" = 1 ]; then
		echo "FAIL: at least one gate failed."
	elif [ "$notrun" = 1 ]; then
		echo "INCOMPLETE: every gate that ran passed, but some did not run (see NOT RUN and skips)."
	else
		echo "PASS: every automated gate ran and passed."
	fi
	echo
	echo "Human UAT (Phase 4, \`docs/uat-phase4-script.md\`), the visual sign-off and the Raspberry Pi run (\`scripts/final-acceptance.sh\`) are separate release evidence and are not claimed here."
} >>"$OUT"

cat "$OUT"
if [ "$failed" = 1 ]; then
	exit 1
fi
if [ "$notrun" = 1 ] && [ "${FINAL_GATE_ALLOW_NOT_RUN:-0}" != 1 ]; then
	exit 2
fi
exit 0
