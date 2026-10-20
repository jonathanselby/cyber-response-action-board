package web

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type fakeRuntime struct {
	bootstrapped bool
	principal    authz.Principal
	picture      domain.OperatingPicture
}

func (f *fakeRuntime) Bootstrapped(context.Context) (bool, error) { return f.bootstrapped, nil }
func (f *fakeRuntime) Bootstrap(context.Context, BootstrapInput) error {
	f.bootstrapped = true
	return nil
}
func (f *fakeRuntime) Authenticate(context.Context, string, string) (string, error) {
	return "token", nil
}
func (f *fakeRuntime) Logout(context.Context, string) error { return nil }
func (f *fakeRuntime) Principal(context.Context, string) (authz.Principal, bool, error) {
	return f.principal, true, nil
}
func (f *fakeRuntime) ListIncidents(context.Context, authz.Principal) ([]IncidentLink, error) {
	return []IncidentLink{{IncidentID: "INC-1", Mode: "exercise", Title: "Bafflewick", Severity: "High"}}, nil
}
func (f *fakeRuntime) PendingDeclarations(context.Context, authz.Principal) ([]PendingDeclaration, error) {
	return nil, nil
}
func (f *fakeRuntime) DeclareIncident(context.Context, authz.Principal, string, string, string, string, string) error {
	return nil
}
func (f *fakeRuntime) Picture(context.Context, authz.Principal, string, string) (domain.OperatingPicture, bool, error) {
	return f.picture, true, nil
}
func (f *fakeRuntime) RecordFact(context.Context, authz.Principal, string, string, string) error {
	return nil
}
func (f *fakeRuntime) ProposeAction(context.Context, authz.Principal, string, domain.Action) error {
	return nil
}
func (f *fakeRuntime) Decide(context.Context, authz.Principal, string, string, string, string, string) error {
	return nil
}

func TestOperatorRequiresBootstrapBeforeLogin(t *testing.T) {
	old := RuntimeFactory
	defer func() { RuntimeFactory = old }()
	RuntimeFactory = func() (Runtime, error) { return &fakeRuntime{}, nil }
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Two-person bootstrap") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestOperatorHomeUsesScopedPrincipal(t *testing.T) {
	old := RuntimeFactory
	defer func() { RuntimeFactory = old }()
	runtime := &fakeRuntime{bootstrapped: true, principal: authz.Principal{Authenticated: true, HumanID: "alice", Role: authz.RoleMajor, Incidents: map[string]bool{"INC-1": true}, Compartments: map[string]map[string]bool{"INC-1": {"GENERAL": true}}, Outputs: map[authz.Action]bool{}}}
	RuntimeFactory = func() (Runtime, error) { return runtime, nil }
	s, err := New()
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(http.MethodGet, "/", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusOK || !strings.Contains(w.Body.String(), "Bafflewick") || strings.Contains(w.Body.String(), "First executable checkpoint") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestOperatorFactoryFailureRefusesServe(t *testing.T) {
	old := RuntimeFactory
	defer func() { RuntimeFactory = old }()
	RuntimeFactory = func() (Runtime, error) { return nil, errors.New("integrity failed") }
	if _, err := New(); err == nil {
		t.Fatal("expected runtime factory error")
	}
}
