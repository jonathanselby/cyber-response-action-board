package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestSlice15OfflineOperatorAcceptance(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)

	if w.Code != http.StatusOK {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
	body := w.Body.String()
	for _, forbidden := range []string{"http://", "https://", "//cdn.", "//fonts."} {
		if strings.Contains(body, forbidden) {
			t.Fatalf("operator UI depends on external asset %q", forbidden)
		}
	}
	for _, required := range []string{
		`<html lang="en">`,
		`name="viewport"`,
		`<label>`,
		`href="/static/app.css"`,
		"Bafflewick",
	} {
		if !strings.Contains(body, required) {
			t.Fatalf("operator UI missing acceptance marker %q", required)
		}
	}

	if got := w.Header().Get("Content-Security-Policy"); !strings.Contains(got, "default-src 'self'") || !strings.Contains(got, "object-src 'none'") {
		t.Fatalf("unexpected content security policy: %q", got)
	}
	if got := w.Header().Get("Cache-Control"); got != "no-store" {
		t.Fatalf("cache control=%q", got)
	}
}

func TestSlice15MutationRequiresAuthenticatedSession(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	r := httptest.NewRequest(http.MethodPost, "/incident", strings.NewReader("incident_id=INC-X"))
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnauthorized {
		t.Fatalf("status=%d body=%q", w.Code, w.Body.String())
	}
}
