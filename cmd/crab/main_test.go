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
	if !strings.HasPrefix(out.String(), "crab 0.0.10-dev") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestReadinessFailsClosedBeforeConfiguredDurableStore(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"readiness"}, &out, &errOut)
	if code != exitRefused {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"ready": false`) || !strings.Contains(out.String(), `"postgresql"`) || !strings.Contains(out.String(), `"authentication_authorisation"`) || !strings.Contains(out.String(), `"incident_semantic_kernel"`) || !strings.Contains(out.String(), `"actions_checklists_timeline"`) || !strings.Contains(out.String(), `"obligations_timed_waits"`) || !strings.Contains(out.String(), `"access_governance_compartments"`) || !strings.Contains(out.String(), `"evidence_immutable_blobs"`) || !strings.Contains(out.String(), `"corpus_deterministic_retrieval"`) || !strings.Contains(out.String(), `"model_assistance"`) {
		t.Fatalf("readiness output = %s", out.String())
	}
}

func TestUnbuiltControlPathRefusesRatherThanFakingCapability(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"setup"}, &out, &errOut)
	if code != exitRefused || !strings.Contains(errOut.String(), "no alternate control path") {
		t.Fatalf("exit=%d stderr=%q", code, errOut.String())
	}
}
