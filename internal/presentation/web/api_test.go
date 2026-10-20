package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

type apiRuntime struct {
	fakeRuntime
	factToken string
	factID    string
	statement string
	source    string
}

func (f *apiRuntime) RecordFact(_ context.Context, token, incidentID, statement, source string) error {
	f.factToken = token
	f.factID = incidentID
	f.statement = statement
	f.source = source
	return nil
}

func TestAPIDeniesMissingBearerWithoutExistenceOracle(t *testing.T) {
	s := withRuntime(t, &apiRuntime{fakeRuntime: fakeRuntime{bootstrapped: true}})
	r := httptest.NewRequest(http.MethodGet, "/api/v1/incidents", nil)
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

func TestAPIUsesExistingRuntimeForStateChange(t *testing.T) {
	runtime := &apiRuntime{fakeRuntime: fakeRuntime{bootstrapped: true}}
	s := withRuntime(t, runtime)
	r := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/INC-1/facts", strings.NewReader(`{"statement":"Service unavailable","source":"operator"}`))
	r.RemoteAddr = "127.0.0.1:12345"
	r.Header.Set("Authorization", "Bearer session-token")
	r.Header.Set("Content-Type", "application/json")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusCreated {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
	if runtime.factToken != "session-token" || runtime.factID != "INC-1" || runtime.statement != "Service unavailable" || runtime.source != "operator" {
		t.Fatalf("runtime call = token=%q id=%q statement=%q source=%q", runtime.factToken, runtime.factID, runtime.statement, runtime.source)
	}
}

func TestAPIMutationRequiresTLSOrLoopback(t *testing.T) {
	s := withRuntime(t, &apiRuntime{fakeRuntime: fakeRuntime{bootstrapped: true}})
	r := httptest.NewRequest(http.MethodPost, "/api/v1/incidents/INC-1/facts", strings.NewReader(`{"statement":"x","source":"y"}`))
	r.RemoteAddr = "203.0.113.10:12345"
	r.Header.Set("Authorization", "Bearer session-token")
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusForbidden {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}
