package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type fakeRuntime struct{ bootstrapped bool }

func (f *fakeRuntime) Bootstrapped(context.Context) (bool, error) { return f.bootstrapped, nil }
func (f *fakeRuntime) Bootstrap(context.Context, BootstrapInput) error {
	f.bootstrapped = true
	return nil
}
func (f *fakeRuntime) Authenticate(context.Context, string, string) (string, error) {
	return "token", nil
}
func (f *fakeRuntime) Logout(context.Context, string) error { return nil }
func (f *fakeRuntime) Home(context.Context, string) (HomeView, bool, error) {
	return HomeView{HumanID: "alice", Role: "Major", CanDeclare: true, Incidents: []IncidentLink{{IncidentID: "INC-1", Mode: "exercise", Title: "Bafflewick", Severity: "High"}}}, true, nil
}
func (f *fakeRuntime) Incident(context.Context, string, string) (IncidentView, bool, error) {
	return IncidentView{IncidentID: "INC-1", Mode: "exercise", Title: "Bafflewick", Severity: "High"}, true, nil
}
func (f *fakeRuntime) DeclareIncident(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) RecordFact(context.Context, string, string, string, string) error { return nil }
func (f *fakeRuntime) ProposeAction(context.Context, string, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) Decide(context.Context, string, string, string, string, string, string) error {
	return nil
}

func withRuntime(t *testing.T, runtime Runtime) *Server {
	t.Helper()
	old := RuntimeFactory
	t.Cleanup(func() { RuntimeFactory = old })
	RuntimeFactory = func() (Runtime, error) { return runtime, nil }
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	return s
}

func TestOperatorStartsWithBootstrap(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Two-person bootstrap") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestAuthenticatedHomeShowsAssignedIncident(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Bafflewick") || strings.Contains(w.Body.String(), "First executable checkpoint") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
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
