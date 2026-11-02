package web

import (
	"net/http"
	"os"
	"strings"
	"testing"
)

// T-237 (C9): each defect found by the browser runs has a test here as well
// as its browser journey.

// F-13: every signed-in page offers Sign out, and the sign-out page ends the
// session only by POST.
func TestSignOutIsOffered(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	if body := get(t, s, "/").Body.String(); !strings.Contains(body, `href="/signout"`) {
		t.Fatal("navigation does not offer Sign out")
	}
	w := get(t, s, "/signout")
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), `<form class="gas-cluster" method="post" action="/logout">`) {
		t.Fatalf("sign-out page code=%d without a POST form", w.Code)
	}
}

// F-14: unsaved input is listed back without hidden routing fields or secrets.
func TestUnsavedFieldsLeaveOutSecretsAndRouting(t *testing.T) {
	got := unsavedFields(map[string][]string{"statement": {"Payroll share encrypted"}, "source": {"EDR"}, "_form": {"record-fact"}, "password": {"x"}, "totp": {"123456"}, "record_id": {"R-1"}, "empty": {"  "}})
	if len(got) != 2 || got[0].Name != "source" || got[1].Value != "Payroll share encrypted" {
		t.Fatalf("unsaved fields = %+v", got)
	}
}

// R-20: the status line wraps rather than cutting off abnormal states.
func TestStatusLineWraps(t *testing.T) {
	raw, err := os.ReadFile("static/crab.css")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), ".gas-status-text{white-space:normal;overflow:visible}") {
		t.Fatal("status line still truncates")
	}
}
