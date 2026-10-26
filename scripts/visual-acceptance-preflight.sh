#!/usr/bin/env bash
set -euo pipefail

ROOT=$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)
cd "$ROOT"

echo "CRAB visual/accessibility preflight"

echo "[1/3] Web semantic, mobile and accessibility contracts"
go test -count=1 ./internal/presentation/web -run '^(TestVisualAccessibilityContract|TestIntakeSurfaceSupportsPhoneCapture|TestOperationsSurfaceShowsContinuityAndGovernedSeverity|TestOperatorNavigationExposesContinuitySurfaces|TestSingleTemplateTrustPoint|TestModeAndExerciseMarkingOnEveryIncidentPage|TestStatusLineShowsAbnormalStatesAsText|TestUnauthorisedIncidentIsNotNamed|TestDecideRefusesUnknownKindBeforeServiceCall|TestDecidePassesExactVersionAndShowsRefusal|TestApprovalShowsChangeInOperatorLanguage|TestNoClientRuntime|TestServiceWorkerBoundary|TestIncidentBriefDownload|TestLocalTimeParsing|TestCompartmentSelectorOffersOnlyReadableCompartments|TestRefusalPreservesInput|TestEveryScreenRendersInShell|TestBoardKeepsFactsUnknownsAndHypothesesDistinct|TestBoardCarriesHandoverSections)$'
go test -count=1 ./internal/architecture -run '^TestTierImportBoundaries$'

echo "[2/3] Print geometry, monochrome/exercise and deterministic rendering"
go test -count=1 ./internal/presentation/pdf -run '^(TestIncidentBriefRendersA4AndLetterLocally|TestExerciseMarkerAndMonochromeDoNotDependOnColour|TestRendererIsDeterministicForFixedInputs)$'

echo "[3/3] Print application composition and source marking"
go test -count=1 ./internal/app -run '^(TestIncidentBriefComposesCurrentStateAndCRABSourceMark|TestRestrictedCompartmentPrintRequiresReadIn)$'

echo "PASS — automated visual/accessibility preflight complete"
echo "Human rendered-browser and printed-page review remains a separate release sign-off."
