package main

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"
	"time"
)

// Marker strings exist only inside the target incident INC-SECRET. No response
// to an unauthorised principal may contain any of them.
var abuseMarkers = []string{"SECRET-TITLE-7F3", "SECRET-FACT-7F3", "SECRET-EVIDENCE-7F3", "SECRET-FORENSIC-7F3", "SECRET-DECISION-7F3", "secret-c2-7f3.example"}

// abuseCase is one registered route with a concrete request against
// INC-SECRET objects and, where the path names an object, the same request
// against made-up objects. A refusal for a real object must look exactly like
// a refusal for an object that does not exist.
type abuseCase struct {
	pattern   string // as registered in internal/presentation/web
	method    string
	path      string
	ghost     string     // same request with nonexistent identifiers; "" when the path names none
	ghostForm url.Values // form for the ghost request when the identifier is in the body
	form      url.Values
	multipart string            // file field for multipart forms ("" = urlencoded)
	api       bool              // JSON API route: bearer token, JSON body in form["json"]
	public    bool              // reachable without sign-in by design
	anonOnly  bool              // run only without a session (it would end the attacker's session)
	mayWrite  map[string]string // principal → why a write is legitimate
}

// abuseFixture holds the target objects and the attacking principals.
type abuseFixture struct {
	evidence, forensic, report, decisionID, decisionVersion, resetID, resetVersion, intakeID, intakeVersion, observable string
	principals                                                                                                          map[string]string // name → session token
}

func multipartBody(t *testing.T, fields url.Values, fileField, filename, content string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for k, vs := range fields {
		for _, v := range vs {
			_ = w.WriteField(k, v)
		}
	}
	if fileField != "" {
		f, err := w.CreateFormFile(fileField, filename)
		if err != nil {
			t.Fatal(err)
		}
		_, _ = f.Write([]byte(content))
	}
	_ = w.Close()
	return &buf, w.FormDataContentType()
}

func (e *e2e) multipart(path, cookie string, fields url.Values, fileField string) *httptest.ResponseRecorder {
	e.t.Helper()
	body, ctype := multipartBody(e.t, fields, fileField, "note.txt", "evidence bytes")
	r := httptest.NewRequest(http.MethodPost, "http://127.0.0.1:8080"+path, body)
	r.RemoteAddr = "127.0.0.1:50000"
	r.Header.Set("Content-Type", ctype)
	if cookie != "" {
		r.AddCookie(&http.Cookie{Name: sessionName, Value: cookie})
	}
	w := httptest.NewRecorder()
	e.handler.ServeHTTP(w, r)
	return w
}

var (
	evidenceLink = regexp.MustCompile(`/evidence/([A-Za-z0-9_-]+)"`)
	reportLink   = regexp.MustCompile(`/intake/([A-Za-z0-9_%-]+)/triage`)
)

func (e *e2e) buildAbuseFixture() abuseFixture {
	e.t.Helper()
	alice, bob := e.bootstrap()
	var f abuseFixture
	e.declare(alice, bob, "INC-SECRET", "SECRET-TITLE-7F3", "Severity level 4")
	e.declare(bob, alice, "INC-OTHER", "Other incident", "Severity level 2")
	e.post("/incident/INC-SECRET/fact", alice, url.Values{"statement": {"SECRET-FACT-7F3"}, "source": {"alice"}})
	if w := e.multipart("/incident/INC-SECRET/evidence?compartment=GENERAL", alice, url.Values{"description": {"SECRET-EVIDENCE-7F3"}, "collector": {"alice"}, "source": {"laptop"}, "current_handler": {"alice"}}, "evidence"); w.Code != http.StatusSeeOther {
		e.t.Fatalf("evidence capture code=%d %s", w.Code, firstError(w.Body.String()))
	}
	m := evidenceLink.FindStringSubmatch(e.get("/incident/INC-SECRET/evidence", alice))
	if m == nil {
		e.t.Fatal("evidence ID not found")
	}
	f.evidence = m[1]
	e.post("/incident/INC-SECRET/access/controller", alice, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}})
	e.approveAll(bob)
	e.post("/incident/INC-SECRET/access/compartment-member", alice, url.Values{"human_id": {"alice"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"lead"}})
	e.approveAll(bob)
	if w := e.multipart("/incident/INC-SECRET/evidence?compartment=FORENSICS", alice, url.Values{"compartment": {"FORENSICS"}, "description": {"SECRET-FORENSIC-7F3"}, "collector": {"alice"}, "source": {"disk"}, "current_handler": {"alice"}}, "evidence"); w.Code != http.StatusSeeOther {
		e.t.Fatalf("forensic evidence capture code=%d %s", w.Code, firstError(w.Body.String()))
	}
	for _, mm := range evidenceLink.FindAllStringSubmatch(e.get("/incident/INC-SECRET/evidence?compartment=FORENSICS", alice), -1) {
		if mm[1] != f.evidence {
			f.forensic = mm[1]
		}
	}
	if f.forensic == "" {
		e.t.Fatal("forensic evidence ID not found")
	}
	// An indicator on INC-SECRET.
	e.post("/incident/INC-SECRET/observables", alice, url.Values{"compartment": {"GENERAL"}, "type": {"domain"}, "value": {"secret-c2-7f3.example"}, "status": {"suspected"}, "source": {"EDR"}})
	if m := regexp.MustCompile(`(OBS-[0-9a-f]+) · version`).FindStringSubmatch(e.get("/incident/INC-SECRET/evidence", alice)); m != nil {
		f.observable = m[1]
	} else {
		e.t.Fatal("observable ID not found")
	}
	// A pending decision on INC-SECRET, visible only to members.
	e.post("/incident/INC-SECRET/decision", alice, url.Values{"decision": {"SECRET-DECISION-7F3", "secret-c2-7f3.example"}, "rationale": {"containment"}})
	// Read the record reference from the maker's own Approvals page.
	ref := regexp.MustCompile(`record ([A-Za-z0-9_-]+) · version ([^<\s]+)`)
	if m := ref.FindStringSubmatch(e.get("/approvals?incident=INC-SECRET", alice)); m != nil {
		f.decisionID, f.decisionVersion = m[1], htmlUnescape(m[2])
	}
	if f.decisionID == "" {
		e.t.Fatal("pending decision not found")
	}
	// A first report and a pending enrolment code.
	if w := e.multipart("/intake", alice, url.Values{"description": {"Suspicious login"}, "reporter": {"helpdesk"}, "source_channel": {"phone"}}, ""); w.Code != http.StatusSeeOther {
		e.t.Fatalf("first report code=%d %s", w.Code, firstError(w.Body.String()))
	}
	if m := reportLink.FindStringSubmatch(e.get("/intake", alice)); m != nil {
		f.report, _ = url.PathUnescape(m[1])
	} else {
		e.t.Fatal("first report ID not found")
	}
	// Attackers: an internal Minor on no incident, a Major on another
	// incident, and an external Minor on another incident.
	f.principals = map[string]string{}
	f.principals["outsider"], _ = e.onboard(alice, bob, "outsider", "Minor", false)
	mallory, _ := e.onboard(alice, bob, "mallory", "Major", false)
	extern, _ := e.onboard(alice, bob, "extern", "Minor", true)
	for _, id := range []string{"mallory", "extern"} {
		e.post("/incident/INC-OTHER/access/member", bob, url.Values{"human_id": {id}, "active": {"true"}, "reason": {"other work"}})
		e.approveAll(alice)
	}
	f.principals["mallory"], f.principals["extern"] = mallory, extern
	e.post("/access/person/reset", alice, url.Values{"human_id": {"outsider"}, "reason": {"pending for abuse test"}})
	for _, form := range regexp.MustCompile(`(?s)<form[^>]*action="/access/reset/confirm".*?</form>`).FindAllString(e.get("/approvals", bob), -1) {
		for _, h := range hiddenField.FindAllStringSubmatch(form, -1) {
			switch h[1] {
			case "record_id":
				f.resetID = h[2]
			case "version":
				f.resetVersion = htmlUnescape(h[2])
			}
		}
	}
	if f.resetID == "" {
		e.t.Fatal("pending reset not found")
	}
	e.post("/access/intake-key", alice, url.Values{"action": {"issue"}, "key_name": {"pending-key"}, "accountable_human": {"alice"}, "expires_at": {e.now().Add(48 * time.Hour).Format("2006-01-02T15:04")}, "reason": {"pending for abuse test"}})
	for _, form := range regexp.MustCompile(`(?s)<form[^>]*action="/access/intake-key/confirm".*?</form>`).FindAllString(e.get("/approvals", bob), -1) {
		for _, h := range hiddenField.FindAllStringSubmatch(form, -1) {
			switch h[1] {
			case "record_id":
				f.intakeID = h[2]
			case "version":
				f.intakeVersion = htmlUnescape(h[2])
			}
		}
	}
	if f.intakeID == "" {
		e.t.Fatal("pending intake key not found")
	}
	return f
}

func abuseCases(f abuseFixture) []abuseCase {
	inc := func(suffix string) string { return "/incident/INC-SECRET" + suffix }
	ghost := func(suffix string) string { return "/incident/INC-NOPE" + suffix }
	majorProposes := map[string]string{"mallory": "Majors may propose organisation-level changes; maker-checker governs them"}
	responders := map[string]string{"outsider": "responders capture first reports by design (PRD §17)", "mallory": "responders capture first reports by design (PRD §17)", "extern": "responders capture first reports by design (PRD §17)"}
	c := []abuseCase{
		// Public by design.
		{pattern: "GET /{$}", method: "GET", path: "/", public: true},
		{pattern: "GET /favicon.ico", method: "GET", path: "/favicon.ico", public: true},
		{pattern: "GET /manifest.webmanifest", method: "GET", path: "/manifest.webmanifest", public: true},
		{pattern: "GET /service-worker.js", method: "GET", path: "/service-worker.js", public: true},
		{pattern: "GET /offline", method: "GET", path: "/offline", public: true},
		{pattern: "GET /healthz", method: "GET", path: "/healthz", public: true},
		{pattern: "GET /static/", method: "GET", path: "/static/crab.css", public: true},
		{pattern: "GET /enrol", method: "GET", path: "/enrol", public: true},
		{pattern: "GET /control", method: "GET", path: "/control", public: true},
		{pattern: "POST /login", method: "POST", path: "/login", form: url.Values{"human_id": {"alice"}, "password": {"guess"}}, public: true},
		{pattern: "POST /logout", method: "POST", path: "/logout", form: url.Values{}, public: true, anonOnly: true},
		{pattern: "POST /setup", method: "POST", path: "/setup", form: url.Values{"prepared_by": {"x"}, "prepared_password": {"Attacker-Pass-1!"}, "confirmed_by": {"y"}, "confirmed_password": {"Attacker-Pass-2!"}}, public: true},
		{pattern: "POST /enrol/begin", method: "POST", path: "/enrol/begin", form: url.Values{"human_id": {"outsider"}, "code": {"AAAAA-AAAAA-AAAAA-AAAAA"}}, public: true},
		{pattern: "POST /enrol/complete", method: "POST", path: "/enrol/complete", form: url.Values{"human_id": {"outsider"}, "code": {"AAAAA-AAAAA-AAAAA-AAAAA"}, "password": {"Attacker-Pass-1!"}}, public: true},
		// Signed-in global screens: allowed, but nothing of INC-SECRET.
		{pattern: "GET /approvals", method: "GET", path: "/approvals?incident=INC-SECRET"},
		{pattern: "GET /access", method: "GET", path: "/access"},
		{pattern: "GET /intake", method: "GET", path: "/intake"},
		{pattern: "GET /obligations", method: "GET", path: "/obligations"},
		{pattern: "GET /organisation", method: "GET", path: "/organisation"},
		{pattern: "GET /readiness", method: "GET", path: "/readiness"},
		{pattern: "GET /pack.pdf", method: "GET", path: "/pack.pdf"},
		{pattern: "GET /api/v1/incidents", method: "GET", path: "/api/v1/incidents", api: true},
		// Incident-scoped reads.
		{pattern: "GET /incident/{incidentID}", method: "GET", path: inc(""), ghost: ghost("")},
		{pattern: "GET /incident/{incidentID}/", method: "GET", path: inc("/"), ghost: ghost("/")},
		{pattern: "GET /incident/{incidentID}/access", method: "GET", path: inc("/access"), ghost: ghost("/access")},
		{pattern: "GET /incident/{incidentID}/brief.pdf", method: "GET", path: inc("/brief.pdf"), ghost: ghost("/brief.pdf")},
		{pattern: "GET /incident/{incidentID}/close", method: "GET", path: inc("/close"), ghost: ghost("/close")},
		{pattern: "GET /incident/{incidentID}/communications", method: "GET", path: inc("/communications"), ghost: ghost("/communications")},
		{pattern: "GET /incident/{incidentID}/evidence", method: "GET", path: inc("/evidence?compartment=FORENSICS"), ghost: ghost("/evidence?compartment=FORENSICS")},
		{pattern: "GET /incident/{incidentID}/pack.pdf", method: "GET", path: inc("/pack.pdf"), ghost: ghost("/pack.pdf")},
		{pattern: "GET /evidence/{evidenceID}", method: "GET", path: "/evidence/" + f.evidence, ghost: "/evidence/EVID-NOPE"},
		{pattern: "GET /evidence/{evidenceID}/attachment", method: "GET", path: "/evidence/" + f.forensic + "/attachment", ghost: "/evidence/EVID-NOPE/attachment"},
		{pattern: "GET /api/v1/incidents/{incidentID}", method: "GET", path: "/api/v1/incidents/INC-SECRET", ghost: "/api/v1/incidents/INC-NOPE", api: true},
		// Incident-scoped writes.
		{pattern: "POST /incident/{incidentID}/fact", method: "POST", path: inc("/fact"), ghost: ghost("/fact"), form: url.Values{"statement": {"planted"}, "source": {"x"}}},
		{pattern: "POST /incident/{incidentID}/uncertainty", method: "POST", path: inc("/uncertainty"), ghost: ghost("/uncertainty"), form: url.Values{"kind": {"unknown"}, "statement": {"planted"}}},
		{pattern: "POST /incident/{incidentID}/decision", method: "POST", path: inc("/decision"), ghost: ghost("/decision"), form: url.Values{"decision": {"planted"}, "rationale": {"x"}}},
		{pattern: "POST /incident/{incidentID}/role", method: "POST", path: inc("/role"), ghost: ghost("/role"), form: url.Values{"role_name": {"Commander"}, "human_id": {"mallory"}}},
		{pattern: "POST /incident/{incidentID}/action", method: "POST", path: inc("/action"), ghost: ghost("/action"), form: url.Values{"title": {"planted"}, "owner": {"mallory"}, "priority": {"high"}}},
		{pattern: "POST /incident/{incidentID}/action/status", method: "POST", path: inc("/action/status"), ghost: ghost("/action/status"), form: url.Values{"action_id": {"ACT-1"}, "status": {"done"}}},
		{pattern: "POST /incident/{incidentID}/relationship", method: "POST", path: inc("/relationship"), ghost: ghost("/relationship"), form: url.Values{"related_incident_id": {"INC-OTHER"}, "reason": {"probe"}, "active": {"true"}}},
		{pattern: "POST /incident/{incidentID}/checklist", method: "POST", path: inc("/checklist"), ghost: ghost("/checklist"), form: url.Values{"template_id": {"T-1"}}},
		{pattern: "POST /incident/{incidentID}/scope", method: "POST", path: inc("/scope"), ghost: ghost("/scope"), form: url.Values{"impact": {"x"}, "confidence": {"low"}, "reason_evidence": {"x"}}},
		{pattern: "POST /incident/{incidentID}/severity", method: "POST", path: inc("/severity"), ghost: ghost("/severity"), form: url.Values{"severity": {"Severity level 1"}, "reason_evidence": {"x"}}},
		{pattern: "POST /incident/{incidentID}/handover", method: "POST", path: inc("/handover"), ghost: ghost("/handover"), form: url.Values{"next_objectives": {"x"}}},
		{pattern: "POST /incident/{incidentID}/resource", method: "POST", path: inc("/resource"), ghost: ghost("/resource"), form: url.Values{"category": {"people"}, "request": {"x"}, "owner": {"mallory"}, "priority": {"high"}, "status": {"requested"}}},
		{pattern: "POST /incident/{incidentID}/obligation/trigger", method: "POST", path: inc("/obligation/trigger"), ghost: ghost("/obligation/trigger"), form: url.Values{"definition_id": {"D-1"}, "trigger_at": {"2026-11-02T00:00"}, "applicability_reason": {"x"}}},
		{pattern: "POST /incident/{incidentID}/obligation/change", method: "POST", path: inc("/obligation/change"), ghost: ghost("/obligation/change"), form: url.Values{"clock_id": {"C-1"}, "kind": {"waive"}, "authority": {"x"}, "reason": {"x"}}},
		{pattern: "POST /incident/{incidentID}/evidence", method: "POST", path: inc("/evidence?compartment=GENERAL"), ghost: ghost("/evidence?compartment=GENERAL"), form: url.Values{"description": {"planted"}}, multipart: "evidence"},
		{pattern: "POST /incident/{incidentID}/communications/draft", method: "POST", path: inc("/communications/draft"), ghost: ghost("/communications/draft"), form: url.Values{"kind": {"situation_report"}, "subject": {"x"}, "body": {"x"}}},
		{pattern: "POST /incident/{incidentID}/communications/issue", method: "POST", path: inc("/communications/issue"), ghost: ghost("/communications/issue"), form: url.Values{"draft_id": {"D-1"}}},
		{pattern: "POST /incident/{incidentID}/communications/cadence", method: "POST", path: inc("/communications/cadence"), ghost: ghost("/communications/cadence"), form: url.Values{"name": {"x"}, "owner": {"mallory"}, "interval_minutes": {"60"}, "first_due_at": {"2026-11-02T00:00"}}},
		{pattern: "POST /incident/{incidentID}/communications/enquiry", method: "POST", path: inc("/communications/enquiry"), ghost: ghost("/communications/enquiry"), form: url.Values{"source": {"press"}, "subject": {"x"}, "message": {"x"}}},
		{pattern: "POST /incident/{incidentID}/communications/sitrep", method: "POST", path: inc("/communications/sitrep"), ghost: ghost("/communications/sitrep"), form: url.Values{"cadence_id": {"C-1"}}},
		{pattern: "POST /incident/{incidentID}/communications/notify", method: "POST", path: inc("/communications/notify"), ghost: ghost("/communications/notify"), form: url.Values{"communication_id": {"COM-1"}}},
		{pattern: "POST /incident/{incidentID}/access/member", method: "POST", path: inc("/access/member"), ghost: ghost("/access/member"), form: url.Values{"human_id": {"mallory"}, "active": {"true"}, "reason": {"self-grant"}}},
		{pattern: "POST /incident/{incidentID}/access/sponsor", method: "POST", path: inc("/access/sponsor"), ghost: ghost("/access/sponsor"), form: url.Values{"human_id": {"mallory"}, "active": {"true"}}},
		{pattern: "POST /incident/{incidentID}/access/controller", method: "POST", path: inc("/access/controller"), ghost: ghost("/access/controller"), form: url.Values{"human_id": {"mallory"}, "compartment_name": {"FORENSICS"}, "active": {"true"}}},
		{pattern: "POST /incident/{incidentID}/access/compartment-member", method: "POST", path: inc("/access/compartment-member"), ghost: ghost("/access/compartment-member"), form: url.Values{"human_id": {"mallory"}, "compartment_name": {"FORENSICS"}, "active": {"true"}, "reason": {"x"}}},
		{pattern: "POST /incident/{incidentID}/close/recovery-update", method: "POST", path: inc("/close/recovery-update"), ghost: ghost("/close/recovery-update"), form: url.Values{"summary": {"x"}, "owner": {"mallory"}}},
		{pattern: "POST /incident/{incidentID}/close/stand-down", method: "POST", path: inc("/close/stand-down"), ghost: ghost("/close/stand-down"), form: url.Values{"reason": {"x"}}},
		{pattern: "POST /incident/{incidentID}/close/pir", method: "POST", path: inc("/close/pir"), ghost: ghost("/close/pir"), form: url.Values{"executive_summary": {"x"}, "lessons": {"x"}}},
		{pattern: "POST /incident/{incidentID}/close/retention", method: "POST", path: inc("/close/retention"), ghost: ghost("/close/retention"), form: url.Values{"retention_class": {"x"}}},
		{pattern: "POST /incident/{incidentID}/close/closure", method: "POST", path: inc("/close/closure"), ghost: ghost("/close/closure"), form: url.Values{"reason": {"x"}, "shareable_summary": {"x"}}},
		{pattern: "POST /incident/{incidentID}/close/reopen", method: "POST", path: inc("/close/reopen"), ghost: ghost("/close/reopen"), form: url.Values{"reason": {"probe"}}},
		{pattern: "POST /incident/{incidentID}/close/transcription", method: "POST", path: inc("/close/transcription"), ghost: ghost("/close/transcription"), form: url.Values{"statement": {"x"}, "paper_occurred_at": {"2026-11-02T00:00"}}},
		{pattern: "POST /evidence/{evidenceID}/handling", method: "POST", path: "/evidence/" + f.evidence + "/handling", ghost: "/evidence/EVID-NOPE/handling", form: url.Values{"action": {"transferred"}, "handler": {"mallory"}}},
		{pattern: "POST /api/v1/incidents/{incidentID}/facts", method: "POST", path: "/api/v1/incidents/INC-SECRET/facts", ghost: "/api/v1/incidents/INC-NOPE/facts", api: true, form: url.Values{"json": {`{"statement":"planted","source":"api"}`}}},
		{pattern: "POST /api/v1/incidents/{incidentID}/actions", method: "POST", path: "/api/v1/incidents/INC-SECRET/actions", ghost: "/api/v1/incidents/INC-NOPE/actions", api: true, form: url.Values{"json": {`{"title":"planted","owner":"mallory","priority":"high"}`}}},
		{pattern: "POST /api/v1/incidents/{incidentID}/decisions", method: "POST", path: "/api/v1/incidents/INC-SECRET/decisions", ghost: "/api/v1/incidents/INC-NOPE/decisions", api: true, form: url.Values{"json": {`{"record_type":"incident.decision","record_id":"` + f.decisionID + `","version":"` + f.decisionVersion + `","outcome":"confirmed"}`}}},
		// Global writes aimed at INC-SECRET objects or Custodian authority.
		{pattern: "POST /decide", method: "POST", path: "/decide", form: url.Values{"kind": {"incident"}, "scope_id": {"INC-SECRET"}, "record_type": {"incident.decision"}, "record_id": {f.decisionID}, "version": {f.decisionVersion}, "outcome": {"confirmed"}}},
		{pattern: "POST /incident", method: "POST", path: "/incident", form: url.Values{"incident_id": {"INC-SECRET"}, "mode": {"exercise"}, "title": {"overwrite"}, "summary": {"overwrite"}, "severity": {"Severity level 1"}}},
		{pattern: "POST /api/v1/incidents", method: "POST", path: "/api/v1/incidents", api: true, form: url.Values{"json": {`{"incident_id":"INC-SECRET","mode":"exercise","title":"overwrite","summary":"overwrite","severity":"Severity level 1"}`}}},
		// Report IDs are legitimately visible to responders; the secret is the
		// incident, so the ghost varies the incident, not the report.
		{pattern: "POST /intake/{reportID}/triage", method: "POST", path: "/intake/" + url.PathEscape(f.report) + "/triage", ghost: "/intake/" + url.PathEscape(f.report) + "/triage", form: url.Values{"decision": {"associate"}, "incident_id": {"INC-SECRET"}, "reason": {"probe"}}, ghostForm: url.Values{"decision": {"associate"}, "incident_id": {"INC-NOPE"}, "reason": {"probe"}}},
		{pattern: "POST /intake", method: "POST", path: "/intake", form: url.Values{"description": {"probe"}, "reporter": {"x"}}, multipart: "-", mayWrite: responders},
		{pattern: "POST /access/output", method: "POST", path: "/access/output", form: url.Values{"human_id": {"mallory"}, "action": {"download"}, "active": {"true"}}},
		{pattern: "POST /access/person", method: "POST", path: "/access/person", form: url.Values{"human_id": {"sock"}, "display_name": {"Sock Puppet"}, "role": {"Major"}, "external": {"false"}}},
		{pattern: "POST /access/person/status", method: "POST", path: "/access/person/status", form: url.Values{"human_id": {"alice"}, "status": {"suspended"}, "reason": {"coup"}}},
		{pattern: "POST /access/person/role", method: "POST", path: "/access/person/role", form: url.Values{"human_id": {"mallory"}, "role": {"Major"}, "remove": {"false"}}},
		{pattern: "POST /access/person/reset", method: "POST", path: "/access/person/reset", form: url.Values{"human_id": {"alice"}, "reason": {"takeover"}}},
		{pattern: "POST /incident/{incidentID}/observables", method: "POST", path: inc("/observables"), ghost: ghost("/observables"), form: url.Values{"compartment": {"GENERAL"}, "type": {"ip"}, "value": {"203.0.113.9"}, "status": {"suspected"}, "source": {"probe"}}},
		{pattern: "POST /observable/{observableID}/status", method: "POST", path: "/observable/" + f.observable + "/status", ghost: "/observable/OBS-NOPE/status", form: url.Values{"status": {"benign"}, "source": {"probe"}}},
		{pattern: "GET /signout", method: "GET", path: "/signout"},
		{pattern: "POST /access/intake-key", method: "POST", path: "/access/intake-key", form: url.Values{"action": {"issue"}, "key_name": {"attacker-key"}, "accountable_human": {"mallory"}, "expires_at": {"2027-01-01T00:00"}, "reason": {"exfiltration"}}},
		{pattern: "POST /access/intake-key/confirm", method: "POST", path: "/access/intake-key/confirm", ghost: "/access/intake-key/confirm", form: url.Values{"record_id": {f.intakeID}, "version": {f.intakeVersion}}, ghostForm: url.Values{"record_id": {"INTAKEKEY-NOPE"}, "version": {f.intakeVersion}}},
		{pattern: "POST /api/v1/first-reports", method: "POST", path: "/api/v1/first-reports", api: true, form: url.Values{"json": {`{"description":"session token posing as an intake key"}`}}},
		{pattern: "POST /access/reset/confirm", method: "POST", path: "/access/reset/confirm", ghost: "/access/reset/confirm", form: url.Values{"record_id": {f.resetID}, "version": {f.resetVersion}}},
		{pattern: "POST /obligations/definition", method: "POST", path: "/obligations/definition", form: url.Values{"name": {"x"}}, mayWrite: majorProposes},
		{pattern: "POST /organisation/severity", method: "POST", path: "/organisation/severity", form: url.Values{"levels": {"a\nb"}}, mayWrite: majorProposes},
		{pattern: "POST /organisation/emergency", method: "POST", path: "/organisation/emergency", form: url.Values{"organisation": {"x"}}, mayWrite: majorProposes},
		{pattern: "POST /organisation/template", method: "POST", path: "/organisation/template", form: url.Values{"name": {"x"}}, mayWrite: majorProposes},
		{pattern: "POST /organisation/window", method: "POST", path: "/organisation/window", form: url.Values{"reason": {"x"}}, mayWrite: majorProposes},
	}
	return c
}

func (e *e2e) abuseRequest(c abuseCase, path, token string) *httptest.ResponseRecorder {
	e.t.Helper()
	switch {
	case c.api:
		return e.api(c.method, path, token, c.form.Get("json"))
	case c.multipart != "":
		field := c.multipart
		if field == "-" {
			field = ""
		}
		return e.multipart(path, token, c.form, field)
	case c.method == http.MethodGet:
		return e.do(http.MethodGet, path, token, nil)
	default:
		return e.do(c.method, path, token, c.form)
	}
}

// T-224: for every route and every unauthorised principal, a request against
// INC-SECRET objects writes nothing, reveals nothing, and is refused exactly
// as a request against objects that do not exist.
func TestAccessControlAbuseSuite(t *testing.T) {
	e := newE2E(t)
	f := e.buildAbuseFixture()
	principals := map[string]string{"anonymous": ""}
	for k, v := range f.principals {
		principals[k] = v
	}
	names := make([]string, 0, len(principals))
	for name := range principals {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, c := range abuseCases(f) {
		// Every attacker must still be signed in, or the case proves nothing.
		for _, who := range names {
			if token := principals[who]; token != "" {
				if w := e.do(http.MethodGet, "/readiness", token, nil); w.Code != http.StatusOK {
					t.Fatalf("before %s: %s is no longer signed in (code %d); the suite would be vacuous", c.pattern, who, w.Code)
				}
			}
		}
		for _, who := range names {
			token := principals[who]
			if c.anonOnly && token != "" {
				continue
			}
			before := e.ledgerCount()
			w := e.abuseRequest(c, c.path, token)
			body := w.Body.String()
			for _, marker := range abuseMarkers {
				if strings.Contains(body, marker) {
					t.Errorf("%s as %s: response reveals %s", c.pattern, who, marker)
				}
			}
			if _, permitted := c.mayWrite[who]; !permitted && !c.public {
				if after := e.ledgerCount(); after != before {
					t.Errorf("%s as %s: %d ledger write(s) from an unauthorised request", c.pattern, who, after-before)
				}
			}
			if c.public && who == "anonymous" && c.method == http.MethodPost {
				if after := e.ledgerCount(); after != before {
					t.Errorf("%s anonymously: wrote to the ledger", c.pattern)
				}
			}
			if c.ghost != "" {
				gc := c
				if c.ghostForm != nil {
					gc.form = c.ghostForm
				}
				g := e.abuseRequest(gc, c.ghost, token)
				if g.Code != w.Code {
					t.Errorf("%s as %s: real object status %d differs from nonexistent object status %d", c.pattern, who, w.Code, g.Code)
				}
			}
		}
	}
}

// T-224 (compartments): a member of INC-SECRET who is not read into FORENSICS
// cannot reach FORENSICS evidence by any route.
func TestCompartmentAbuseByIncidentMember(t *testing.T) {
	e := newE2E(t)
	f := e.buildAbuseFixture()
	alice := e.login("alice", e2eAlicePassword)
	bob := e.login("bob", e2eBobPassword)
	genny, _ := e.onboard(alice, bob, "genny", "Minor", false)
	e.post("/incident/INC-SECRET/access/member", alice, url.Values{"human_id": {"genny"}, "active": {"true"}, "reason": {"general responder"}})
	e.approveAll(bob)
	if !strings.Contains(e.get("/incident/INC-SECRET", genny), "SECRET-TITLE-7F3") {
		t.Fatal("genny cannot see INC-SECRET at all")
	}
	for _, req := range []struct{ path, ghost string }{
		{"/evidence/" + f.forensic, "/evidence/EVID-NOPE"},
		{"/evidence/" + f.forensic + "/attachment", "/evidence/EVID-NOPE/attachment"},
		{"/incident/INC-SECRET/evidence?compartment=FORENSICS", "/incident/INC-SECRET/evidence?compartment=NOPE"},
	} {
		w := e.do(http.MethodGet, req.path, genny, nil)
		g := e.do(http.MethodGet, req.ghost, genny, nil)
		if strings.Contains(w.Body.String(), "SECRET-FORENSIC-7F3") {
			t.Errorf("%s reveals FORENSICS evidence", req.path)
		}
		if w.Code == http.StatusOK || w.Code != g.Code {
			t.Errorf("%s status %d (nonexistent %d)", req.path, w.Code, g.Code)
		}
	}
	before := e.ledgerCount()
	e.do(http.MethodPost, "/evidence/"+f.forensic+"/handling", genny, url.Values{"action": {"transferred"}, "handler": {"genny"}})
	if e.ledgerCount() != before {
		t.Error("handling record written for FORENSICS evidence without read-in")
	}
}

// T-225: every route registered by the browser and API tiers has an abuse
// case, so a new route cannot be added without one.
func TestEveryRouteHasAnAbuseCase(t *testing.T) {
	_, here, _, _ := runtime.Caller(0)
	webDir := filepath.Join(filepath.Dir(here), "..", "..", "internal", "presentation", "web")
	files, err := filepath.Glob(filepath.Join(webDir, "*.go"))
	if err != nil {
		t.Fatal(err)
	}
	route := regexp.MustCompile(`(?:HandleFunc|mux\.Handle)\("([A-Z]+ [^"]+)"`)
	registered := map[string]bool{}
	for _, file := range files {
		if strings.HasSuffix(file, "_test.go") {
			continue
		}
		b, err := os.ReadFile(file)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range route.FindAllStringSubmatch(string(b), -1) {
			registered[m[1]] = true
		}
	}
	covered := map[string]bool{}
	for _, c := range abuseCases(abuseFixture{}) {
		covered[c.pattern] = true
	}
	var missing []string
	for r := range registered {
		if !covered[r] {
			missing = append(missing, r)
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("routes without an abuse case (add one to abuseCases):\n%s", strings.Join(missing, "\n"))
	}
	if len(registered) < 80 {
		t.Fatalf("only %d routes found; the route scan is broken", len(registered))
	}
}
