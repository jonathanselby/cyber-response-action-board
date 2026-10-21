package web

import (
	"context"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
)

type fakeContinuityRuntime struct {
	*fakeRuntime
	severityProposed bool
}

func (f *fakeContinuityRuntime) Intake(context.Context, string) (IntakeView, bool, error) {
	return IntakeView{HumanID: "alice", Role: "Major", CanTriage: true, Reports: []IntakeReportRow{{ReportID: "REPORT-1", Description: "Phone report", SourceChannel: "phone", ReceivedAt: "2026-10-21T01:00:00Z"}}}, true, nil
}
func (f *fakeContinuityRuntime) CaptureFirstReport(context.Context, string, FirstReportCapture) error {
	return nil
}
func (f *fakeContinuityRuntime) TriageFirstReport(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *fakeContinuityRuntime) Operations(context.Context, string, string) (OperationsView, bool, error) {
	return OperationsView{
		Scopes:          []ScopeAssessmentRow{{Impact: "Payroll unavailable", Confidence: "high", ReasonEvidence: "SOC evidence"}},
		PendingSeverity: []PendingSeverityRow{{RecordID: "SEV-1", Version: "v1", Maker: "alice", Severity: "Critical", ReasonEvidence: "Spread confirmed"}},
		Resources:       []ResourceRequestRow{{Category: "people", Request: "IR surge", OwnerHumanID: "alice", Priority: "P1", Status: "requested", AuthoritySource: "incident lead"}},
	}, true, nil
}
func (f *fakeContinuityRuntime) RecordScopeAssessment(context.Context, string, string, ScopeAssessmentInput) error {
	return nil
}
func (f *fakeContinuityRuntime) ProposeSeverityChange(context.Context, string, string, string, string) error {
	f.severityProposed = true
	return nil
}
func (f *fakeContinuityRuntime) DecideSeverity(context.Context, string, string, string, string, string) error {
	return nil
}
func (f *fakeContinuityRuntime) GenerateHandover(context.Context, string, string, []string, []string) error {
	return nil
}
func (f *fakeContinuityRuntime) RecordResourceRequest(context.Context, string, string, ResourceRequestInput) error {
	return nil
}

func TestIntakeSurfaceSupportsPhoneCapture(t *testing.T) {
	s := withRuntime(t, &fakeContinuityRuntime{fakeRuntime: &fakeRuntime{bootstrapped: true}})
	r := httptest.NewRequest(http.MethodGet, "/intake", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "First reports") || !strings.Contains(body, `capture="environment"`) || !strings.Contains(body, "Phone report") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}
}

func TestOperationsSurfaceShowsContinuityAndGovernedSeverity(t *testing.T) {
	runtime := &fakeContinuityRuntime{fakeRuntime: &fakeRuntime{bootstrapped: true}}
	s := withRuntime(t, runtime)
	r := httptest.NewRequest(http.MethodGet, "/incident/INC-1/operations", nil)
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	body := w.Body.String()
	if w.Code != http.StatusOK || !strings.Contains(body, "Scope &amp; impact") || !strings.Contains(body, "Proposed: Critical") || !strings.Contains(body, "IR surge") {
		t.Fatalf("code=%d body=%q", w.Code, body)
	}

	form := url.Values{"severity": {"Critical"}, "reason_evidence": {"Spread confirmed"}}
	post := httptest.NewRequest(http.MethodPost, "/incident/INC-1/operations/severity", strings.NewReader(form.Encode()))
	post.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	post.Header.Set("Origin", "http://example.com")
	post.Host = "example.com"
	post.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	postWriter := httptest.NewRecorder()
	s.Handler().ServeHTTP(postWriter, post)
	if postWriter.Code != http.StatusSeeOther || !runtime.severityProposed {
		t.Fatalf("code=%d proposed=%v", postWriter.Code, runtime.severityProposed)
	}
}

func TestOperatorNavigationExposesContinuitySurfaces(t *testing.T) {
	s := withRuntime(t, &fakeContinuityRuntime{fakeRuntime: &fakeRuntime{bootstrapped: true}})
	home := httptest.NewRequest(http.MethodGet, "/", nil)
	home.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	homeWriter := httptest.NewRecorder()
	s.Handler().ServeHTTP(homeWriter, home)
	if !strings.Contains(homeWriter.Body.String(), `href="/intake"`) {
		t.Fatalf("home missing intake navigation: %q", homeWriter.Body.String())
	}

	incident := httptest.NewRequest(http.MethodGet, "/incident/INC-1", nil)
	incident.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	incidentWriter := httptest.NewRecorder()
	s.Handler().ServeHTTP(incidentWriter, incident)
	if !strings.Contains(incidentWriter.Body.String(), `href="/incident/INC-1/operations"`) {
		t.Fatalf("incident missing operations navigation: %q", incidentWriter.Body.String())
	}
}
