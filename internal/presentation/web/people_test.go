package web

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

// C2 web tests assert rendering and refusal mapping only. Every rule is
// decided by the runtime (application tier) and tested there.

func TestPeopleSectionRendersWhatTheRuntimeDecided(t *testing.T) {
	f := &fakeRuntime{bootstrapped: true, people: []PersonRow{
		{HumanID: "jane", DisplayName: "Jane Doe", Organisation: "Forensic Co", Status: "active", Role: "Minor", External: true, Usable: true, AccountExpiresAt: "2 Nov 2026 08:17 UTC", Incidents: []string{"INC-004", "INC-007"}, CanSuspend: true, CanDisable: true, CanReset: true},
		{HumanID: "old", DisplayName: "old", Status: "suspended", Role: "Major", CanReactivate: true},
	}}
	body := get(t, withRuntime(t, f), "/access").Body.String()
	for _, want := range []string{"Jane Doe", "EXTERNAL", "Forensic Co", "2 Nov 2026 08:17 UTC", "INC-004, INC-007", "Propose enrolment code", "Propose suspension", "Propose disabling", "Propose reactivation", `action="/access/person"`} {
		if !strings.Contains(body, want) {
			t.Errorf("people section missing %q", want)
		}
	}
	// Actions the runtime did not offer are not rendered.
	// jane may be suspended and disabled but not reactivated; old may only be
	// reactivated. Each action appears exactly as often as it was offered.
	for action, want := range map[string]int{"Propose reactivation": 1, "Propose suspension": 1, "Propose disabling": 1, "Propose enrolment code": 1} {
		if got := strings.Count(body, action); got != want {
			t.Errorf("%q rendered %d times, want %d", action, got, want)
		}
	}
}

func TestAddPersonPassesFormToRuntime(t *testing.T) {
	f := &fakeRuntime{bootstrapped: true}
	w := postForm(t, withRuntime(t, f), "/access/person", url.Values{"human_id": {"jane"}, "display_name": {"Jane Doe"}, "organisation": {"Forensic Co"}, "role": {"Minor"}, "external": {"true"}, "account_expires_at": {"2026-11-02T08:17"}})
	if w.Code != http.StatusSeeOther {
		t.Fatalf("add person code=%d", w.Code)
	}
	p := f.lastPerson
	if p.HumanID != "jane" || !p.External || p.Role != "Minor" || p.AccountExpiresAt == nil || p.AccountExpiresAt.Format("15:04") != "08:17" {
		t.Fatalf("person input=%+v", p)
	}
}

func TestSignInOffersAuthenticatorAndEnrolment(t *testing.T) {
	f := &fakeRuntime{bootstrapped: true}
	body := anon(t, withRuntime(t, f), http.MethodGet, "/", nil).Body.String()
	for _, want := range []string{`name="totp"`, `autocomplete="one-time-code"`, `href="/enrol"`} {
		if !strings.Contains(body, want) {
			t.Errorf("sign-in missing %q", want)
		}
	}
	w := anonPost(t, f, "/login", url.Values{"human_id": {"newcomer"}, "password": {"GOOD-CODE"}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/enrol?human_id=newcomer" {
		t.Fatalf("enrolment code at sign-in: code=%d location=%q", w.Code, w.Header().Get("Location"))
	}
}

func TestEnrolmentFlowRendersTOTPOnlyForExternalPeople(t *testing.T) {
	f := &fakeRuntime{bootstrapped: true}
	ext := anonPost(t, f, "/enrol/begin", url.Values{"human_id": {"external"}, "code": {"GOOD-CODE"}})
	if ext.Code != http.StatusOK || !strings.Contains(ext.Body.String(), "JBSWY3DPEHPK3PXPJBSWY3DPEHPK3PXP") || !strings.Contains(ext.Body.String(), `name="totp"`) {
		t.Fatalf("external enrolment step: code=%d", ext.Code)
	}
	if cc := ext.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("enrolment step carrying the code is cacheable: %q", cc)
	}
	internal := anonPost(t, f, "/enrol/begin", url.Values{"human_id": {"jane"}, "code": {"GOOD-CODE"}})
	if strings.Contains(internal.Body.String(), `name="totp"`) {
		t.Fatal("internal enrolment asked for an authenticator")
	}
	bad := anonPost(t, f, "/enrol/begin", url.Values{"human_id": {"jane"}, "code": {"WRONG"}})
	if bad.Code != http.StatusUnauthorized || !strings.Contains(bad.Body.String(), "not accepted") {
		t.Fatalf("bad code: code=%d", bad.Code)
	}
	weak := anonPost(t, f, "/enrol/complete", url.Values{"human_id": {"jane"}, "code": {"GOOD-CODE"}, "password": {"short"}})
	if weak.Code != http.StatusUnprocessableEntity || !strings.Contains(weak.Body.String(), "12 characters") {
		t.Fatalf("weak password: code=%d", weak.Code)
	}
	done := anonPost(t, f, "/enrol/complete", url.Values{"human_id": {"jane"}, "code": {"GOOD-CODE"}, "password": {"Forensic-Night-26!"}})
	if done.Code != http.StatusSeeOther || !strings.Contains(done.Header().Get("Set-Cookie"), sessionCookie+"=token") {
		t.Fatalf("complete enrolment: code=%d cookie=%q", done.Code, done.Header().Get("Set-Cookie"))
	}
}

func TestResetCodeIsShownInTheBodyOnly(t *testing.T) {
	f := &fakeRuntime{bootstrapped: true}
	w := postForm(t, withRuntime(t, f), "/access/reset/confirm", url.Values{"record_id": {"RESET-1"}, "version": {"v1"}})
	if w.Code != http.StatusOK {
		t.Fatalf("confirm reset code=%d", w.Code)
	}
	if !strings.Contains(w.Body.String(), "ABCDE-FGHIJ-KLMNO-PQRST") || !strings.Contains(w.Body.String(), "Shown once") {
		t.Fatal("reset code page missing code or warning")
	}
	if loc := w.Header().Get("Location"); strings.Contains(loc, "ABCDE") {
		t.Fatal("reset code leaked into a URL")
	}
	if cc := w.Header().Get("Cache-Control"); !strings.Contains(cc, "no-store") {
		t.Fatalf("reset code page cacheable: %q", cc)
	}
}

func anon(t *testing.T, s *Server, method, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	var body *strings.Reader
	if form != nil {
		body = strings.NewReader(form.Encode())
	} else {
		body = strings.NewReader("")
	}
	r := httptest.NewRequest(method, "http://example.com"+path, body)
	if form != nil {
		r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		r.Header.Set("Origin", "http://example.com")
	}
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	return w
}

func anonPost(t *testing.T, f *fakeRuntime, path string, form url.Values) *httptest.ResponseRecorder {
	t.Helper()
	return anon(t, withRuntime(t, f), http.MethodPost, path, form)
}
