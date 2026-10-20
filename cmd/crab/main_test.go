package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestVersionCommand(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"version"}, &out, &errOut)
	if code != exitOK {
		t.Fatalf("exit = %d stderr=%s", code, errOut.String())
	}
	if !strings.HasPrefix(out.String(), "crab 0.0.12-dev") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestReadinessFailsClosedBeforeConfiguredDurableStore(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"readiness"}, &out, &errOut)
	if code != exitRefused {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"ready": false`) || !strings.Contains(out.String(), `"postgresql"`) || !strings.Contains(out.String(), `"authentication_authorisation"`) || !strings.Contains(out.String(), `"incident_semantic_kernel"`) || !strings.Contains(out.String(), `"actions_checklists_timeline"`) || !strings.Contains(out.String(), `"obligations_timed_waits"`) || !strings.Contains(out.String(), `"access_governance_compartments"`) || !strings.Contains(out.String(), `"evidence_immutable_blobs"`) || !strings.Contains(out.String(), `"corpus_deterministic_retrieval"`) || !strings.Contains(out.String(), `"model_assistance"`) || !strings.Contains(out.String(), `"communications_notifier"`) || !strings.Contains(out.String(), `"print_paper_resilience"`) {
		t.Fatalf("readiness output = %s", out.String())
	}
}

func TestSetupIsPartOfTheImplementedCLISurface(t *testing.T) {
	var out bytes.Buffer
	usage(&out)
	text := out.String()
	if !strings.Contains(text, "crab setup") || !strings.Contains(text, "crab serve") {
		t.Fatalf("usage does not expose setup through the shared operator server: %s", text)
	}
}

func TestPrintCommandRemainsFailClosedUntilAuthenticatedOperatorWiringExists(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"print"}, &out, &errOut)
	if code != exitRefused || !strings.Contains(errOut.String(), "no alternate control path") {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
}
