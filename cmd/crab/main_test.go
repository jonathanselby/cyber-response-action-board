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
	if !strings.HasPrefix(out.String(), "crab 0.0.4-dev") {
		t.Fatalf("version output = %q", out.String())
	}
}

func TestReadinessFailsClosedBeforeConfiguredDurableStore(t *testing.T) {
	var out, errOut bytes.Buffer
	code := run([]string{"readiness"}, &out, &errOut)
	if code != exitRefused {
		t.Fatalf("exit = %d stdout=%s stderr=%s", code, out.String(), errOut.String())
	}
	if !strings.Contains(out.String(), `"ready": false`) || !strings.Contains(out.String(), `"postgresql"`) || !strings.Contains(out.String(), `"authentication_authorisation"`) || !strings.Contains(out.String(), `"incident_semantic_kernel"`) {
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
