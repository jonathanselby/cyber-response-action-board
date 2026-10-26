package web

import (
	"errors"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

// decisionKinds is the closed set accepted by POST /decide (UI notes 6.4).
var decisionKinds = map[string]bool{
	"declaration": true, "incident": true, "severity": true, "obligation": true,
	"communication": true, "access": true, "recovery": true,
}

var (
	incidentsScreen = screen{nav: "incidents", title: "Command Summary", summary: "What needs your attention now."}
	approvalsScreen = screen{nav: "approvals", title: "Approvals", summary: "Changes waiting for a second person.", tmpl: "approvals"}
	intakeScreen    = screen{nav: "intake", title: "First reports", summary: "Capture what was reported before anyone declares an incident.", tmpl: "intake"}
	readinessScreen = screen{nav: "readiness", title: "Readiness", summary: "Will CRAB work when it is needed?", tmpl: "readiness"}
	libraryScreen   = screen{nav: "obligations", title: "Obligation library", summary: "Organisation-defined obligations and their clocks.", tmpl: "obligations"}
	adminScreen     = screen{nav: "access-admin", title: "Access admin", summary: "Identity and access metadata. No incident content is shown here.", tmpl: "access-admin"}
)

func (s *Server) registerSystemRoutes() {
	incidents := incidentsScreen
	incidents.tmpl = "incidents"
	incidents.load = s.loadIncidents
	approvals := approvalsScreen
	approvals.load = s.loadApprovals
	intake := intakeScreen
	intake.load = s.loadIntake
	readiness := readinessScreen
	readiness.load = func(r *http.Request, token string, _ FrameView) (any, []Action, error) {
		v, err := s.runtime.Readiness(r.Context(), token)
		return v, nil, err
	}
	library := libraryScreen
	library.load = func(r *http.Request, token string, _ FrameView) (any, []Action, error) {
		v, err := s.runtime.GlobalObligations(r.Context(), token)
		return v, []Action{{Label: "Propose obligation", URL: "?open=propose#propose", Primary: true}}, err
	}
	admin := adminScreen
	admin.load = func(r *http.Request, token string, _ FrameView) (any, []Action, error) {
		v, err := s.runtime.AccessAdministration(r.Context(), token, time.Now().UTC())
		return v, []Action{{Label: "Grant print or download", URL: "?open=output#output", Primary: true}}, err
	}

	s.mux.HandleFunc("GET /{$}", s.home(incidents))
	s.mux.HandleFunc("POST /setup", s.setup)
	s.mux.HandleFunc("POST /login", s.login)
	s.mux.HandleFunc("POST /logout", s.logout)
	s.mux.HandleFunc("POST /incident", s.post(incidents, func(*http.Request) string { return "/" }, "proposed", func(r *http.Request, token string) error {
		return s.runtime.DeclareIncident(r.Context(), token, formValue(r, "incident_id"), formValue(r, "mode"), formValue(r, "title"), formValue(r, "summary"), formValue(r, "severity"))
	}))

	s.mux.HandleFunc("GET /approvals", s.show(approvals))
	s.mux.HandleFunc("POST /decide", s.post(approvals, func(r *http.Request) string {
		if id := formValue(r, "return_incident"); id != "" {
			return "/approvals?incident=" + url.QueryEscape(id)
		}
		return "/approvals"
	}, "decided", s.decide))

	s.mux.HandleFunc("GET /intake", s.show(intake))
	s.mux.HandleFunc("POST /intake", s.post(intake, func(*http.Request) string { return "/intake" }, "report-recorded", s.captureFirstReport))
	s.mux.HandleFunc("POST /intake/{reportID}/triage", s.post(intake, func(*http.Request) string { return "/intake" }, "triaged", func(r *http.Request, token string) error {
		return s.runtime.TriageFirstReport(r.Context(), token, r.PathValue("reportID"), formValue(r, "decision"), formValue(r, "incident_id"), formValue(r, "reason"))
	}))

	s.mux.HandleFunc("GET /readiness", s.show(readiness))
	s.mux.HandleFunc("GET /obligations", s.show(library))
	s.mux.HandleFunc("POST /obligations/definition", s.post(library, func(*http.Request) string { return "/obligations" }, "proposed", s.proposeObligationDefinition))
	s.mux.HandleFunc("GET /access", s.show(admin))
	s.mux.HandleFunc("POST /access/output", s.post(admin, func(*http.Request) string { return "/access" }, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeOutputPermission(r.Context(), token, formValue(r, "human_id"), formValue(r, "action"), formValue(r, "active") != "false")
	}))

	// Removed global screens redirect for one release (UI notes 8.1).
	s.mux.HandleFunc("GET /control", func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, "/", http.StatusMovedPermanently) })
}

// home serves bootstrap, sign-in or the Incidents screen.
func (s *Server) home(incidents screen) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ready, err := s.runtime.Bootstrapped(r.Context())
		if err != nil {
			s.page(w, http.StatusServiceUnavailable, screen{title: "CRAB unavailable", tmpl: "refused"}, pageData{Error: refusalText(err)}, nil)
			return
		}
		if !ready {
			s.page(w, http.StatusOK, screen{title: "First-run setup", summary: "Two different people create the first accounts together.", tmpl: "setup"}, pageData{Notice: messages[r.URL.Query().Get("m")]}, nil)
			return
		}
		token := sessionToken(r)
		if token == "" {
			s.page(w, http.StatusOK, screen{title: "Sign in", tmpl: "signin"}, pageData{}, nil)
			return
		}
		s.render(w, r, token, incidents, http.StatusOK, pageData{Notice: messages[r.URL.Query().Get("m")]})
	}
}

func (s *Server) loadIncidents(r *http.Request, token string, _ FrameView) (any, []Action, error) {
	home, ok, err := s.runtime.Home(r.Context(), token)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errors.New("authentication required")
	}
	var actions []Action
	if home.CanDeclare {
		actions = append(actions, Action{Label: "Declare incident", URL: "?open=declare#declare", Primary: true})
	}
	return home, actions, nil
}

func (s *Server) setup(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) || !sameOrigin(r) {
		http.Error(w, "setup refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input := BootstrapInput{PreparedBy: formValue(r, "prepared_by"), PreparedPassword: r.FormValue("prepared_password"), ConfirmedBy: formValue(r, "confirmed_by"), ConfirmedPassword: r.FormValue("confirmed_password"), SeverityLevels: lines(r.FormValue("severity_levels"))}
	setupScreen := screen{title: "First-run setup", summary: "Two different people create the first accounts together.", tmpl: "setup"}
	keep := url.Values{"prepared_by": {input.PreparedBy}, "confirmed_by": {input.ConfirmedBy}, "severity_levels": {r.FormValue("severity_levels")}}
	if input.PreparedBy == "" || input.ConfirmedBy == "" || input.PreparedBy == input.ConfirmedBy {
		s.page(w, http.StatusUnprocessableEntity, setupScreen, pageData{Error: "Two distinct people are required.", Form: keep}, nil)
		return
	}
	// The password rule lives in the security tier (T-193); its refusal is shown here.
	if err := s.runtime.Bootstrap(r.Context(), input); err != nil {
		s.page(w, http.StatusUnprocessableEntity, setupScreen, pageData{Error: refusalText(err), Form: keep}, nil)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "login refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, err := s.runtime.Authenticate(r.Context(), formValue(r, "human_id"), r.FormValue("password"))
	if err != nil {
		s.page(w, http.StatusUnauthorized, screen{title: "Sign in", tmpl: "signin"}, pageData{Error: "Sign-in failed. Check your ID and password.", Form: url.Values{"human_id": {formValue(r, "human_id")}}}, nil)
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "logout refused", http.StatusForbidden)
		return
	}
	if token := sessionToken(r); token != "" {
		_ = s.runtime.Logout(r.Context(), token)
	}
	s.clearSession(w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) loadApprovals(r *http.Request, token string, _ FrameView) (any, []Action, error) {
	view, err := s.runtime.Approvals(r.Context(), token, strings.TrimSpace(r.URL.Query().Get("incident")))
	return view, nil, err
}

// decide is the single browser decision path. The kind is checked against a
// closed set before any service call; the application tier re-loads the
// stored proposal, binds the exact version and refuses maker-as-confirmer.
func (s *Server) decide(r *http.Request, token string) error {
	kind := formValue(r, "kind")
	if !decisionKinds[kind] {
		return errors.New("unknown approval kind")
	}
	outcome := formValue(r, "outcome")
	if outcome != "confirmed" && outcome != "rejected" {
		return errors.New("outcome must be confirm or reject")
	}
	input := DecisionInput{Kind: kind, ScopeID: formValue(r, "scope_id"), RecordType: formValue(r, "record_type"), RecordID: formValue(r, "record_id"), Version: formValue(r, "version"), Outcome: outcome}
	if input.RecordID == "" || input.Version == "" || input.RecordType == "" {
		return errors.New("the approval reference is incomplete")
	}
	return s.runtime.DecideProposal(r.Context(), token, input)
}

func (s *Server) captureFirstReport(r *http.Request, token string) error {
	r.Body = http.MaxBytesReader(nil, r.Body, firstReportRequestBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		return fieldError{"the report could not be read"}
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	received, err := parseLocalTime(r.FormValue("received_at"), "Received time", false)
	if err != nil {
		return err
	}
	input := FirstReportCapture{
		Reporter:        formValue(r, "reporter"),
		ContactRoute:    formValue(r, "contact_route"),
		TimeConfidence:  formValue(r, "time_confidence"),
		Description:     formValue(r, "description"),
		Affected:        formValue(r, "affected"),
		ImmediateImpact: formValue(r, "immediate_impact"),
		SourceChannel:   formValue(r, "source_channel"),
	}
	if received != nil {
		input.ReceivedAt = *received
	}
	var closers []io.Closer
	defer func() {
		for _, c := range closers {
			_ = c.Close()
		}
	}()
	for _, field := range []string{"camera", "audio", "video", "attachment"} {
		for _, header := range r.MultipartForm.File[field] {
			if header == nil || strings.TrimSpace(header.Filename) == "" {
				continue
			}
			file, err := header.Open()
			if err != nil {
				return fieldError{"an attachment could not be opened"}
			}
			closers = append(closers, file)
			input.Attachments = append(input.Attachments, FirstReportAttachment{Filename: header.Filename, MediaType: header.Header.Get("Content-Type"), Reader: file})
		}
	}
	return s.runtime.CaptureFirstReport(r.Context(), token, input)
}

func (s *Server) loadIntake(r *http.Request, token string, _ FrameView) (any, []Action, error) {
	view, ok, err := s.runtime.Intake(r.Context(), token)
	if err != nil {
		return nil, nil, err
	}
	if !ok {
		return nil, nil, errors.New("authentication required")
	}
	home, _, err := s.runtime.Home(r.Context(), token)
	if err != nil {
		return nil, nil, err
	}
	return struct {
		Intake    IntakeView
		Incidents []IncidentLink
	}{view, home.Incidents}, nil, nil
}

func (s *Server) proposeObligationDefinition(r *http.Request, token string) error {
	duration, err := strconv.Atoi(formValue(r, "duration"))
	if err != nil || duration < 0 {
		return fieldError{"Duration must be zero or a positive whole number"}
	}
	var warnings []int
	for _, line := range lines(r.FormValue("warnings")) {
		n, err := strconv.Atoi(line)
		if err != nil || n <= 0 {
			return fieldError{"Warning minutes must be positive whole numbers, one per line"}
		}
		warnings = append(warnings, n)
	}
	def := domain.ObligationDefinition{
		Name:                   formValue(r, "name"),
		TriggerField:           formValue(r, "trigger_field"),
		Duration:               duration,
		Unit:                   formValue(r, "unit"),
		Calendar:               domain.BusinessCalendar{Timezone: formValue(r, "timezone"), Weekend: lines(r.FormValue("weekend")), Holidays: lines(r.FormValue("holidays")), Adjustment: formValue(r, "adjustment")},
		ApplicabilityCondition: formValue(r, "applicability"),
		Recipient:              formValue(r, "recipient"),
		Owner:                  formValue(r, "owner"),
		AuthoritySource:        formValue(r, "authority"),
		WarningBeforeMinutes:   warnings,
	}
	return s.runtime.ProposeObligationDefinition(r.Context(), token, def)
}
