package web

import (
	"context"
	"strings"
	"testing"
)

// T-199: the Command Summary is built from the frame call and the Home view
// only; it does not fan out to other listings.
func TestCommandSummaryUsesFrameAndHomeOnly(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true, status: StatusFlags{PackStale: true, TimeStatus: "trusted"}}
	s := withRuntime(t, runtime)
	runtime.reads = nil
	body := get(t, s, "/").Body.String()
	if runtime.reads["Frame"] != 1 || runtime.reads["Home"] != 1 || len(runtime.reads) != 2 {
		t.Fatalf("summary reads=%v", runtime.reads)
	}
	for _, want := range []string{
		"<h1>Command Summary</h1>", "What needs your attention now.",
		"Needs attention", "2</strong> approvals waiting for your decision", `href="/approvals"`,
		"1</strong> first report awaiting triage", `href="/intake"`,
		"Emergency Response Pack stale", `href="/readiness#pack"`,
		"My active incidents", "Bafflewick payroll", "Severity: High", "Phase: Active response",
		"Only items you are authorised to see are shown.", "Declare incident",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("summary missing %q", want)
		}
	}
}

// The summary states calmly when nothing is waiting.
func TestCommandSummaryEmptyStates(t *testing.T) {
	s := withRuntime(t, &emptyHomeRuntime{fakeRuntime{bootstrapped: true}})
	body := get(t, s, "/").Body.String()
	for _, want := range []string{"Nothing needs your attention.", "You are not assigned to any incident."} {
		if !strings.Contains(body, want) {
			t.Fatalf("missing empty state %q", want)
		}
	}
}

type emptyHomeRuntime struct{ fakeRuntime }

func (f *emptyHomeRuntime) Home(ctx context.Context, token string) (HomeView, bool, error) {
	return HomeView{HumanID: "alice", Role: "Minor"}, true, nil
}
