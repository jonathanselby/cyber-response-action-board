package web

import (
	"strings"
	"testing"
)

// T-195: an unverified host clock is stated on every authenticated page,
// including Approvals, without stopping work.
func TestUnverifiedTimeIsStatedOnEveryPage(t *testing.T) {
	for _, status := range []string{"untrusted", "drifting"} {
		s := withRuntime(t, &fakeRuntime{bootstrapped: true, status: StatusFlags{TimeStatus: status}})
		for _, path := range []string{"/", "/approvals", "/incident/" + testIncident} {
			w := get(t, s, path)
			if w.Code != 200 || !strings.Contains(w.Body.String(), "TIME UNVERIFIED") {
				t.Fatalf("%s status=%s code=%d: missing TIME UNVERIFIED notice", path, status, w.Code)
			}
		}
	}
	s := withRuntime(t, &fakeRuntime{bootstrapped: true, status: StatusFlags{TimeStatus: "trusted"}})
	if body := get(t, s, "/").Body.String(); strings.Contains(body, "CRAB cannot verify this host") {
		t.Fatal("trusted clock must not show the unverified notice")
	}
}
