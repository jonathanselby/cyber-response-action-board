package web

import (
	"context"
	"errors"
	"net/http"
	"net/url"
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

type outageRuntime struct{ fakeRuntime }

func (o *outageRuntime) RecordFact(context.Context, string, string, string, string) error {
	return errors.New("connect: connection refused")
}

func (o *outageRuntime) Frame(context.Context, string, string) (FrameView, bool, error) {
	return FrameView{}, false, errors.New("connect: connection refused")
}

// T-241 (found by the database harness): when CRAB cannot reach PostgreSQL a
// request gets a plain 503 saying nothing was saved, and the session cookie is
// kept so the person is still signed in when the database returns.
func TestDatabaseOutageKeepsTheSession(t *testing.T) {
	s := withRuntime(t, &outageRuntime{fakeRuntime{bootstrapped: true}})
	w := postForm(t, s, "/incident/INC-1/fact", url.Values{"statement": {"Typed during the outage"}, "source": {"EDR"}})
	if w.Code != http.StatusServiceUnavailable || !strings.Contains(w.Body.String(), "nothing was saved") || !strings.Contains(w.Body.String(), "Typed during the outage") {
		t.Fatalf("outage response code=%d", w.Code)
	}
	for _, c := range w.Result().Cookies() {
		if c.Name == sessionCookie && c.MaxAge < 0 {
			t.Fatal("the session cookie was cleared during a database outage")
		}
	}
}
