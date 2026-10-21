#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

if [ -z "${CRAB_TEST_POSTGRES_ADMIN_URL:-}" ]; then
  echo "CRAB_TEST_POSTGRES_ADMIN_URL is required for Bafflewick acceptance" >&2
  exit 1
fi

OUT=${1:-}
TMP=$(mktemp -d)
trap 'rm -rf "$TMP"' EXIT
EVIDENCE="$TMP/bafflewick-acceptance.md"
PASS_COUNT=0
REQUIRED_PHASES=8

{
  echo "# CRAB Bafflewick acceptance evidence"
  echo
  echo "- UTC run: $(date -u +%Y-%m-%dT%H:%M:%SZ)"
  if command -v git >/dev/null 2>&1; then
    echo "- git commit: $(git rev-parse HEAD)"
  fi
  echo "- threshold: ${REQUIRED_PHASES}/${REQUIRED_PHASES} mandatory phases"
  echo
} >"$EVIDENCE"

run_phase() {
  local number=$1
  local name=$2
  local expected_csv=$3
  local regex=$4
  shift 4
  local log="$TMP/phase-${number}.log"

  echo "Bafflewick ${number}/${REQUIRED_PHASES}: ${name}" >&2
  if ! go test -count=1 -v "$@" -run "^(${regex})$" >"$log" 2>&1; then
    cat "$log" >&2
    {
      echo "## Phase ${number} — ${name}: FAIL"
      echo
      echo "A required executable proof failed."
      echo
    } >>"$EVIDENCE"
    [ -z "$OUT" ] || cp "$EVIDENCE" "$OUT"
    exit 1
  fi

  IFS=',' read -r -a expected <<<"$expected_csv"
  for test_name in "${expected[@]}"; do
    if ! grep -Fq "=== RUN   ${test_name}" "$log"; then
      cat "$log" >&2
      echo "required Bafflewick proof did not run: ${test_name}" >&2
      {
        echo "## Phase ${number} — ${name}: FAIL"
        echo
        echo "Required proof missing: \`${test_name}\`."
        echo
      } >>"$EVIDENCE"
      [ -z "$OUT" ] || cp "$EVIDENCE" "$OUT"
      exit 1
    fi
  done

  PASS_COUNT=$((PASS_COUNT + 1))
  {
    echo "## Phase ${number} — ${name}: PASS"
    echo
    for test_name in "${expected[@]}"; do
      echo "- \`${test_name}\`"
    done
    echo
  } >>"$EVIDENCE"
}

run_phase 1 "intake, live operation, actions and handover" \
  "TestContinuityProductFlowThroughLedgerAndPostgres,TestConfiguredChecklistActivatesAsOrdinaryActionProposals" \
  "TestContinuityProductFlowThroughLedgerAndPostgres|TestConfiguredChecklistActivatesAsOrdinaryActionProposals" \
  ./internal/data/postgres ./internal/app

run_phase 2 "scoped access, external expiry/revocation and rich evidence" \
  "TestAccessGovernanceSponsorControllerCustodianAndExpiry,TestAccessProjectionAndPrincipalAreDenyByDefault,TestEvidenceExactBytesIntegrityScopeAndSeparateDownloadPermission,TestEvidenceHandlingHistoryAndRegisterRebuildWithoutTouchingBlobs" \
  "TestAccessGovernanceSponsorControllerCustodianAndExpiry|TestAccessProjectionAndPrincipalAreDenyByDefault|TestEvidenceExactBytesIntegrityScopeAndSeparateDownloadPermission|TestEvidenceHandlingHistoryAndRegisterRebuildWithoutTouchingBlobs" \
  ./internal/data/postgres

run_phase 3 "obligation clocks, situation reports and enquiries" \
  "TestObligationTriggerMovesProvisionalToFirmWithoutChangingDeadline,TestObligationExtensionAndWaiverPreserveOriginalDeadline,TestCommunicationIssueCadenceNotificationAndRebuild,TestIncomingEnquiryAssignmentPersistsAndRebuilds" \
  "TestObligationTriggerMovesProvisionalToFirmWithoutChangingDeadline|TestObligationExtensionAndWaiverPreserveOriginalDeadline|TestCommunicationIssueCadenceNotificationAndRebuild|TestIncomingEnquiryAssignmentPersistsAndRebuilds" \
  ./internal/data/postgres

run_phase 4 "model loss and corporate identity loss" \
  "TestModelUnavailableKeepsSourcesAndDoesNotReroute,TestModelQueueShedsNewGenerationWithoutBlockingCoreWork,TestSecurityStoreLocalAuthAndSessions,TestIsolateModeRefusesFederationBeforeVerification" \
  "TestModelUnavailableKeepsSourcesAndDoesNotReroute|TestModelQueueShedsNewGenerationWithoutBlockingCoreWork|TestSecurityStoreLocalAuthAndSessions|TestIsolateModeRefusesFederationBeforeVerification" \
  ./internal/app ./internal/data/postgres ./internal/security/...

run_phase 5 "paper fallback, database loss and reconciliation" \
  "TestPaperTranscriptionTimelineAndRebuildPreserveBothTimes,TestGeneratedDerivedFileRemainsReadableWithoutDatabase,TestIncidentBriefComposesCurrentStateAndCRABSourceMark,TestIncidentBriefRendersA4AndLetterLocally" \
  "TestPaperTranscriptionTimelineAndRebuildPreserveBothTimes|TestGeneratedDerivedFileRemainsReadableWithoutDatabase|TestIncidentBriefComposesCurrentStateAndCRABSourceMark|TestIncidentBriefRendersA4AndLetterLocally" \
  ./internal/data/postgres ./internal/app ./internal/adapters/artifacts ./internal/presentation/pdf

run_phase 6 "backup, restore mechanics, rebuild and independent ledger verification" \
  "TestBackupCreatesHashMetadataAndDoesNotUseShell,TestRestoreRefusesTamperedBackupBeforePostgres,TestRestoreUsesPostgresNativeTooling,TestAppendReplayRollbackTamperAndExport,TestVerifyValidLedger,TestVerifyDetectsTamper" \
  "TestBackupCreatesHashMetadataAndDoesNotUseShell|TestRestoreRefusesTamperedBackupBeforePostgres|TestRestoreUsesPostgresNativeTooling|TestAppendReplayRollbackTamperAndExport|TestVerifyValidLedger|TestVerifyDetectsTamper" \
  ./internal/data/postgres ./internal/ledger

run_phase 7 "recovery, stand-down, PIR, retention and closure" \
  "TestRecoveryStandDownPIRRetentionClosureAndRebuild,TestClosedIncidentLedgerGateDoesNotBlockSystemScope" \
  "TestRecoveryStandDownPIRRetentionClosureAndRebuild|TestClosedIncidentLedgerGateDoesNotBlockSystemScope" \
  ./internal/data/postgres

run_phase 8 "attack and degraded-output controls" \
  "TestEvidencePartialOversizeAndCorruptionFailClosed,TestExerciseExternalNotificationSuppressedBeforeAdapterIOInPostgresPath,TestManagedModelSensitivityRefusesBeforeProviderCall,TestRestrictedCompartmentPrintRequiresReadIn" \
  "TestEvidencePartialOversizeAndCorruptionFailClosed|TestExerciseExternalNotificationSuppressedBeforeAdapterIOInPostgresPath|TestManagedModelSensitivityRefusesBeforeProviderCall|TestRestrictedCompartmentPrintRequiresReadIn" \
  ./internal/data/postgres ./internal/app

if [ "$PASS_COUNT" -ne "$REQUIRED_PHASES" ]; then
  echo "Bafflewick threshold failed: ${PASS_COUNT}/${REQUIRED_PHASES}" >&2
  exit 1
fi

{
  echo "## Result"
  echo
  echo "PASS — ${PASS_COUNT}/${REQUIRED_PHASES} mandatory Bafflewick phases passed."
  echo
  echo "This gate proves the deterministic software scenario. Physical Raspberry Pi measurement and human visual review remain separate release evidence."
} >>"$EVIDENCE"

cat "$EVIDENCE"
if [ -n "$OUT" ]; then
  cp "$EVIDENCE" "$OUT"
  echo "Bafflewick evidence written to $OUT" >&2
fi
