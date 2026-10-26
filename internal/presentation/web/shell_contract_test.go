package web

// Slice 15 presentation contract tests (TEST-ACCEPTANCE-NOTES §3).

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// T-167: exactly one place converts executed templates into trusted HTML.
func TestSingleTemplateTrustPoint(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range files {
		if strings.HasSuffix(name, "_test.go") {
			continue
		}
		src, err := os.ReadFile(name)
		if err != nil {
			t.Fatal(err)
		}
		count := strings.Count(string(src), "template.HTML(")
		if name == "page.go" {
			if count == 0 || count > 2 {
				t.Fatalf("page.go should hold the single conversion point; found %d", count)
			}
			continue
		}
		if count > 0 {
			t.Fatalf("%s converts to template.HTML outside page.go", name)
		}
		if strings.Contains(string(src), "<main>") || strings.Contains(string(src), "<!doctype") {
			t.Fatalf("%s builds page markup outside templates", name)
		}
	}
}

// T-168, T-169: mode is text in the persistent header; exercise pages carry
// the exercise notice in the body.
func TestModeAndExerciseMarkingOnEveryIncidentPage(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true, mode: "exercise"})
	for _, path := range []string{"/incident/INC-1", "/incident/INC-1/evidence", "/incident/INC-1/communications", "/incident/INC-1/access", "/incident/INC-1/close", "/evidence/EV-1"} {
		body := get(t, s, path).Body.String()
		if !strings.Contains(body, "EXERCISE · INC-1") {
			t.Fatalf("%s: header context missing exercise mode text", path)
		}
		if !strings.Contains(body, "no real obligations start and no external notifications are sent") {
			t.Fatalf("%s: exercise notice missing", path)
		}
	}
	live := withRuntime(t, &fakeRuntime{bootstrapped: true, mode: "live"})
	body := get(t, live, "/incident/INC-1").Body.String()
	if !strings.Contains(body, "LIVE · INC-1") || strings.Contains(body, "no real obligations start") {
		t.Fatal("live incident must say LIVE and carry no exercise notice")
	}
}

// T-170: abnormal states appear as text in the persistent status line.
func TestStatusLineShowsAbnormalStatesAsText(t *testing.T) {
	healthy := withRuntime(t, &fakeRuntime{bootstrapped: true})
	if body := get(t, healthy, "/readiness").Body.String(); !strings.Contains(body, `<span class="gas-status-text">Ready</span>`) {
		t.Fatal("healthy status line should read Ready")
	}
	runtime := &fakeRuntime{bootstrapped: true, status: StatusFlags{Isolate: true, BreakGlass: true, TimeStatus: "untrusted", PendingApprovals: 3}}
	s := withRuntime(t, runtime)
	body := get(t, s, "/readiness").Body.String()
	for _, want := range []string{"Break-glass active", "Isolate on", "Time unverified", "3 approvals"} {
		if !strings.Contains(body, want) {
			t.Fatalf("status line missing %q", want)
		}
	}
	if got := statusLine(FrameView{Status: StatusFlags{IntegrityFailed: true, BreakGlass: true, Isolate: true, TimeStatus: "drifting", PackStale: true}}); !strings.HasSuffix(got, "+1") {
		t.Fatalf("status line should truncate after four items: %q", got)
	}
}

// T-171, T-186: an incident the user may not see is never named, and the
// refusal stays inside the frame.
func TestUnauthorisedIncidentIsNotNamed(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	w := get(t, s, "/incident/INC-SECRET")
	body := w.Body.String()
	if w.Code != http.StatusNotFound {
		t.Fatalf("code=%d", w.Code)
	}
	if strings.Contains(body, "INC-SECRET") {
		t.Fatal("refusal leaked the requested incident identifier")
	}
	if !strings.Contains(body, "gas-header") || !strings.Contains(body, "not available to you") {
		t.Fatal("refusal must render inside the application frame")
	}
	if strings.Contains(body, `href="/incident/INC-SECRET`) {
		t.Fatal("navigation leaked the unauthorised incident")
	}
}

// T-172: /decide refuses unknown kinds before any runtime call.
func TestDecideRefusesUnknownKindBeforeServiceCall(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	w := postForm(t, s, "/decide", url.Values{"kind": {"anything"}, "record_type": {"incident.action"}, "record_id": {"A"}, "version": {"v1"}, "outcome": {"confirmed"}})
	if w.Code == http.StatusSeeOther || runtime.decided {
		t.Fatalf("unknown kind reached the runtime: code=%d", w.Code)
	}
	w = postForm(t, s, "/decide", url.Values{"kind": {"severity"}, "record_type": {"incident.severity_change"}, "record_id": {"SEV-1"}, "version": {"v1"}, "outcome": {"approve-everything"}})
	if runtime.decided {
		t.Fatal("unknown outcome reached the runtime")
	}
}

// T-173, T-174: /decide passes the exact proposal reference to the single
// decision path; app-tier refusals are shown, not swallowed.
func TestDecidePassesExactVersionAndShowsRefusal(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	w := postForm(t, s, "/decide", url.Values{"kind": {"severity"}, "scope_id": {"INC-1"}, "record_type": {"incident.severity_change"}, "record_id": {"SEV-1"}, "version": {"v7"}, "outcome": {"confirmed"}, "return_incident": {"INC-1"}})
	if w.Code != http.StatusSeeOther || w.Header().Get("Location") != "/approvals?incident=INC-1&m=decided" {
		t.Fatalf("code=%d location=%q", w.Code, w.Header().Get("Location"))
	}
	if runtime.decision != (DecisionInput{Kind: "severity", ScopeID: "INC-1", RecordType: "incident.severity_change", RecordID: "SEV-1", Version: "v7", Outcome: "confirmed"}) {
		t.Fatalf("decision = %+v", runtime.decision)
	}
	runtime.refuse = errors.New("maker cannot confirm own proposal")
	w = postForm(t, s, "/decide", url.Values{"kind": {"severity"}, "scope_id": {"INC-1"}, "record_type": {"incident.severity_change"}, "record_id": {"SEV-1"}, "version": {"v7"}, "outcome": {"confirmed"}})
	if w.Code == http.StatusSeeOther || !strings.Contains(w.Body.String(), "Maker cannot confirm own proposal") {
		t.Fatalf("refusal not shown: code=%d", w.Code)
	}
}

// T-176: the approval surface shows what changes, in operator language.
func TestApprovalShowsChangeInOperatorLanguage(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	body := get(t, s, "/approvals").Body.String()
	for _, want := range []string{"Change severity to Critical", "Severity change", "<td>High</td>", "<strong>Critical</strong>", "Proposed by <strong>bob</strong>", "Spread to second share confirmed", "If confirmed:", "Confirm", "Reject", "Technical reference"} {
		if !strings.Contains(body, want) {
			t.Fatalf("approval missing %q", want)
		}
	}
	if strings.Contains(body, ">incident.severity_change<") || strings.Contains(body, "Approve Item") {
		t.Fatal("internal record type shown as the decision surface")
	}
}

// T-177, T-178: core flows are plain HTML forms; no CRAB JavaScript exists.
func TestNoClientRuntime(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	scriptTag := regexp.MustCompile(`<script[^>]*>`)
	for _, path := range []string{"/", "/approvals", "/intake", "/incident/INC-1", "/incident/INC-1/evidence", "/incident/INC-1/close", "/readiness"} {
		body := get(t, s, path).Body.String()
		for _, tag := range scriptTag.FindAllString(body, -1) {
			if !strings.Contains(tag, `src="/_app-shell/pwa.js"`) {
				t.Fatalf("%s: unexpected script %s", path, tag)
			}
		}
		if strings.Contains(body, " onclick=") || strings.Contains(body, " onsubmit=") || strings.Contains(body, "style=\"") {
			t.Fatalf("%s: inline script or style found", path)
		}
	}
	entries, err := os.ReadDir("static")
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".js") {
			t.Fatalf("CRAB-authored JavaScript found: %s", e.Name())
		}
	}
}

// T-179: the service worker precaches only static assets and never caches
// authenticated routes; pages stay no-store.
func TestServiceWorkerBoundary(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	worker := get(t, s, "/service-worker.js").Body.String()
	m := regexp.MustCompile(`const PRECACHE = (\[[^\]]*\]);`).FindStringSubmatch(worker)
	if len(m) != 2 {
		t.Fatalf("precache list not found in worker")
	}
	for _, forbidden := range []string{"/incident", "/evidence", "/approvals", "/api", "/intake"} {
		if strings.Contains(m[1], forbidden) {
			t.Fatalf("precache contains authenticated route %s", forbidden)
		}
	}
	for _, required := range []string{"/offline", "/_app-shell/shell.css", "/static/crab.css"} {
		if !strings.Contains(m[1], required) {
			t.Fatalf("precache missing %s", required)
		}
	}
	if !strings.Contains(worker, `if (event.request.method !== "GET") return;`) {
		t.Fatal("worker must ignore non-GET requests")
	}
	page := get(t, s, "/incident/INC-1")
	if page.Header().Get("Cache-Control") != "no-store" {
		t.Fatal("authenticated pages must be no-store")
	}
	offline := get(t, s, "/offline").Body.String()
	if strings.Contains(offline, "INC-1") || !strings.Contains(offline, "Emergency Response Pack") {
		t.Fatal("offline page must be static paper guidance with no incident data")
	}
}

// T-180: paper downloads are attachments; refusals stay in frame.
func TestIncidentBriefDownload(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	w := get(t, s, "/incident/INC-1/brief.pdf?size=A4&mode=monochrome")
	if w.Code != http.StatusOK || w.Header().Get("Content-Type") != "application/pdf" || !strings.HasPrefix(w.Header().Get("Content-Disposition"), "attachment;") || w.Header().Get("Cache-Control") != "no-store" {
		t.Fatalf("code=%d headers=%v", w.Code, w.Header())
	}
	runtime.pdfErr = errors.New("authorisation denied")
	w = get(t, s, "/incident/INC-1/brief.pdf")
	if w.Code != http.StatusForbidden || !strings.Contains(w.Body.String(), "gas-header") {
		t.Fatalf("refused print should render in frame: code=%d", w.Code)
	}
}

// T-182: datetime-local values are parsed as labelled UTC or refused.
func TestLocalTimeParsing(t *testing.T) {
	got, err := parseLocalTime("2026-10-26T09:30", "When", true)
	if err != nil || got.Format("2006-01-02T15:04Z07:00") != "2026-10-26T09:30Z" {
		t.Fatalf("got %v err %v", got, err)
	}
	if _, err := parseLocalTime("", "When", true); err == nil {
		t.Fatal("required blank time accepted")
	}
	if v, err := parseLocalTime("", "When", false); err != nil || v != nil {
		t.Fatal("optional blank time should be nil")
	}
	if _, err := parseLocalTime("26/10/2026", "When", false); err == nil {
		t.Fatal("malformed time accepted")
	}
}

// T-183: the compartment selector offers only readable compartments.
func TestCompartmentSelectorOffersOnlyReadableCompartments(t *testing.T) {
	general := withRuntime(t, &fakeRuntime{bootstrapped: true})
	if body := get(t, general, "/incident/INC-1/evidence").Body.String(); strings.Contains(body, `<option value="LEGAL"`) {
		t.Fatal("LEGAL offered to a user not read into it")
	}
	runtime := &fakeRuntime{bootstrapped: true, compartments: []string{"GENERAL", "LEGAL"}}
	s := withRuntime(t, runtime)
	body := get(t, s, "/incident/INC-1/evidence?compartment=LEGAL").Body.String()
	if !strings.Contains(body, `<option value="LEGAL" selected>`) || runtime.evidenceComp != "LEGAL" || !strings.Contains(body, "Viewing LEGAL.") {
		t.Fatalf("read-in compartment not offered or not in scope summary (evidence compartment %q)", runtime.evidenceComp)
	}
}

// T-186: a refused form re-renders in frame with the user's input kept.
func TestRefusalPreservesInput(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true, refuse: errors.New("authorisation denied")}
	s := withRuntime(t, runtime)
	w := postForm(t, s, "/incident/INC-1/fact", url.Values{"_form": {"record-fact"}, "statement": {"Backups intact"}, "source": {"Ops"}})
	body := w.Body.String()
	if w.Code != http.StatusForbidden || !strings.Contains(body, "Backups intact") || !strings.Contains(body, "gas-error-summary") || !strings.Contains(body, `id="record-fact" open`) {
		t.Fatalf("code=%d", w.Code)
	}
}

// AC-68: evidence capture accepts camera, audio or file in one multipart form.
func TestEvidenceCaptureUsesPickedInput(t *testing.T) {
	runtime := &fakeRuntime{bootstrapped: true}
	s := withRuntime(t, runtime)
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	part, _ := mw.CreateFormFile("camera", "photo.jpg")
	_, _ = part.Write([]byte("jpeg"))
	for k, v := range map[string]string{"collector": "carol", "source": "laptop", "current_handler": "carol", "compartment": "GENERAL"} {
		_ = mw.WriteField(k, v)
	}
	_ = mw.Close()
	r := httptest.NewRequest(http.MethodPost, "http://example.com/incident/INC-1/evidence", &buf)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r.Header.Set("Origin", "http://example.com")
	r.AddCookie(&http.Cookie{Name: sessionCookie, Value: "token"})
	w := httptest.NewRecorder()
	s.Handler().ServeHTTP(w, r)
	if w.Code != http.StatusSeeOther || !runtime.called("CaptureEvidence") {
		t.Fatalf("code=%d body=%q", w.Code, w.Body.String())
	}
}

// Every screen renders inside the shell for an authorised user.
func TestEveryScreenRendersInShell(t *testing.T) {
	s := withRuntime(t, &fakeRuntime{bootstrapped: true})
	for _, path := range []string{"/", "/approvals", "/intake", "/readiness", "/obligations", "/access", "/incident/INC-1", "/incident/INC-1/evidence", "/incident/INC-1/communications", "/incident/INC-1/access", "/incident/INC-1/close", "/evidence/EV-1", "/incident/INC-1?q=isolate"} {
		w := get(t, s, path)
		body := w.Body.String()
		if w.Code != http.StatusOK {
			t.Fatalf("%s: code=%d body=%q", path, w.Code, body)
		}
		for _, want := range []string{"gas-skip-link", `<nav class="gas-nav"`, `class="gas-main"`, "/static/crab.css?v="} {
			if !strings.Contains(body, want) {
				t.Fatalf("%s: shell frame missing %q", path, want)
			}
		}
		if strings.Contains(body, "could not be displayed") {
			t.Fatalf("%s: template failed to render", path)
		}
		for _, internal := range []string{"RecordType", "record_type:", "drop2", "requested capability is not available"} {
			if strings.Contains(body, internal) {
				t.Fatalf("%s: internal term %q in primary UX", path, internal)
			}
		}
	}
}
