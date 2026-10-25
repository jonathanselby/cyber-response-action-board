package web

import (
	"errors"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

func TestOperatorStartsWithBootstrap(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "First-run setup") || !strings.Contains(body, "must be different people") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}
}

func TestOperatorSetupAcceptsEquivalentLoopbackOrigin(t *testing.T) {
	runtime := &fakeRuntime{}
	s := withRuntime(t, runtime)
	form := url.Values{
		"prepared_by":        {"alice"},
		"prepared_password":  {"alice-password-123"},
		"confirmed_by":       {"bob"},
		"confirmed_password": {"bob-password-12345"},
	}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/setup", strings.NewReader(form.Encode()))
	r.RemoteAddr = "127.0.0.1:45000"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "http://localhost:8080")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || !runtime.bootstrapped {
		t.Fatalf("code=%d bootstrapped=%v body=%q", w.Code, runtime.bootstrapped, w.Body.String())
	}
}

func TestOperatorSetupRefusesNonLocalOrigin(t *testing.T) {
	runtime := &fakeRuntime{}
	s := withRuntime(t, runtime)
	form := url.Values{
		"prepared_by":        {"alice"},
		"prepared_password":  {"alice-password-123"},
		"confirmed_by":       {"bob"},
		"confirmed_password": {"bob-password-12345"},
	}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/setup", strings.NewReader(form.Encode()))
	r.RemoteAddr = "127.0.0.1:45000"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	r.Header.Set("Origin", "https://example.invalid")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden || runtime.bootstrapped {
		t.Fatalf("code=%d bootstrapped=%v body=%q", w.Code, runtime.bootstrapped, w.Body.String())
	}
}

func TestOperatorSetupRefusesSamePersonTwice(t *testing.T) {
	runtime := &fakeRuntime{}
	s := withRuntime(t, runtime)
	form := url.Values{"prepared_by": {"alice"}, "prepared_password": {"alice-password-123"}, "confirmed_by": {"alice"}, "confirmed_password": {"alice-password-123"}}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/setup", strings.NewReader(form.Encode()))
	r.RemoteAddr = "127.0.0.1:45000"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusUnprocessableEntity || runtime.bootstrapped || !strings.Contains(w.Body.String(), "Two distinct people") {
		t.Fatalf("code=%d bootstrapped=%v body=%q", w.Code, runtime.bootstrapped, w.Body.String())
	}
}

func TestAuthenticatedHomeShowsAssignedIncident(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	w := get(t, s, "/")
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "Bafflewick payroll") || strings.Contains(body, "First executable checkpoint") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}
	for _, want := range []string{"Declare incident", "2 to decide", `href="/approvals?incident=INC-1"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("incidents screen missing %q", want)
		}
	}
}

func TestSignInFailureStaysInFrame(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	form := url.Values{"human_id": {"wrong"}, "password": {"x"}}
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080/login", strings.NewReader(form.Encode()))
	r.RemoteAddr = "127.0.0.1:45000"
	r.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != http.StatusUnauthorized || !strings.Contains(body, "Sign-in failed") || !strings.Contains(body, `value="wrong"`) || !strings.Contains(body, "gas-header") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}
}

func TestRuntimeFactoryFailureRefusesStart(t *testing.T) {
	old := RuntimeFactory
	defer func() { RuntimeFactory = old }()
	RuntimeFactory = func() (Runtime, error) { return nil, errors.New("integrity failed") }
	if _, err := New(); err == nil {
		t.Fatal("expected runtime factory error")
	}
}
