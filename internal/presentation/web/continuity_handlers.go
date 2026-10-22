package web

import (
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	appshell "github.com/jonathanselby/go-app-shell/shell"
)

type continuityPage struct {
	Version    string
	Error      string
	Intake     *IntakeView
	Incident   *IncidentView
	Operations *OperationsView
}

func (s *Server) registerContinuityRoutes() {
	s.mux.HandleFunc("GET /intake", s.continuityIntake)
	s.mux.HandleFunc("POST /intake", s.continuityCapture)
	s.mux.HandleFunc("POST /intake/{reportID}/triage", s.continuityTriage)
	s.mux.HandleFunc("GET /incident/{incidentID}/operations", s.continuityOperations)
	s.mux.HandleFunc("POST /incident/{incidentID}/operations/scope", s.continuityScope)
	s.mux.HandleFunc("POST /incident/{incidentID}/operations/severity", s.continuitySeverity)
	s.mux.HandleFunc("POST /incident/{incidentID}/operations/severity/decide", s.continuitySeverityDecision)
	s.mux.HandleFunc("POST /incident/{incidentID}/operations/handover", s.continuityHandover)
	s.mux.HandleFunc("POST /incident/{incidentID}/operations/resource", s.continuityResource)
}

func (s *Server) continuityCapability(w http.ResponseWriter) (ContinuityRuntime, bool) {
	runtime, ok := s.runtime.(ContinuityRuntime)
	if !ok {
		http.Error(w, "continuity surface unavailable", http.StatusNotImplemented)
	}
	return runtime, ok
}

func (s *Server) continuityIntake(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok {
		return
	}
	token := sessionToken(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, found, err := runtime.Intake(r.Context(), token)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !found {
		s.clearSession(w)
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	s.renderContinuity(w, continuityPage{Intake: &view})
}

func (s *Server) continuityCapture(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok {
		return
	}
	if sessionToken(r) == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	if !sameOrigin(r) {
		http.Error(w, "request origin refused", http.StatusForbidden)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, firstReportRequestBytes)
	if err := r.ParseMultipartForm(1 << 20); err != nil {
		http.Error(w, "invalid multipart form", http.StatusBadRequest)
		return
	}
	if r.MultipartForm != nil {
		defer r.MultipartForm.RemoveAll()
	}
	input := FirstReportCapture{
		Reporter:        strings.TrimSpace(r.FormValue("reporter")),
		ContactRoute:    strings.TrimSpace(r.FormValue("contact_route")),
		TimeConfidence:  strings.TrimSpace(r.FormValue("time_confidence")),
		Description:     strings.TrimSpace(r.FormValue("description")),
		Affected:        strings.TrimSpace(r.FormValue("affected")),
		ImmediateImpact: strings.TrimSpace(r.FormValue("immediate_impact")),
		SourceChannel:   strings.TrimSpace(r.FormValue("source_channel")),
	}
	if raw := strings.TrimSpace(r.FormValue("received_at")); raw != "" {
		parsed, err := time.ParseInLocation("2006-01-02T15:04", raw, time.Local)
		if err != nil {
			http.Error(w, "invalid received time", http.StatusBadRequest)
			return
		}
		input.ReceivedAt = parsed
	}
	var closers []io.Closer
	for _, field := range []string{"camera", "attachment"} {
		if r.MultipartForm == nil {
			break
		}
		for _, header := range r.MultipartForm.File[field] {
			if header == nil || strings.TrimSpace(header.Filename) == "" {
				continue
			}
			file, err := header.Open()
			if err != nil {
				http.Error(w, "attachment could not be opened", http.StatusBadRequest)
				return
			}
			closers = append(closers, file)
			input.Attachments = append(input.Attachments, FirstReportAttachment{Filename: header.Filename, MediaType: header.Header.Get("Content-Type"), Reader: file})
		}
	}
	defer func() {
		for _, closer := range closers {
			_ = closer.Close()
		}
	}()
	if err := runtime.CaptureFirstReport(r.Context(), sessionToken(r), input); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/intake", http.StatusSeeOther)
}

func (s *Server) continuityTriage(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	if err := runtime.TriageFirstReport(r.Context(), sessionToken(r), r.PathValue("reportID"), strings.TrimSpace(r.FormValue("decision")), strings.TrimSpace(r.FormValue("incident_id")), strings.TrimSpace(r.FormValue("reason"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/intake", http.StatusSeeOther)
}

func (s *Server) continuityOperations(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok {
		return
	}
	token := sessionToken(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	incidentID := r.PathValue("incidentID")
	incident, found, err := s.runtime.Incident(r.Context(), token, incidentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	operations, found, err := runtime.Operations(r.Context(), token, incidentID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !found {
		http.NotFound(w, r)
		return
	}
	s.renderContinuity(w, continuityPage{Incident: &incident, Operations: &operations})
}

func (s *Server) continuityScope(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	input := ScopeAssessmentInput{AffectedAssets: r.FormValue("affected_assets"), AffectedServices: r.FormValue("affected_services"), PeopleDataPopulations: r.FormValue("people_data_populations"), Locations: r.FormValue("locations"), Suppliers: r.FormValue("suppliers"), Impact: strings.TrimSpace(r.FormValue("impact")), Confidence: strings.TrimSpace(r.FormValue("confidence")), ReasonEvidence: strings.TrimSpace(r.FormValue("reason_evidence"))}
	if err := runtime.RecordScopeAssessment(r.Context(), sessionToken(r), r.PathValue("incidentID"), input); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.redirectOperations(w, r)
}

func (s *Server) continuitySeverity(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	if err := runtime.ProposeSeverityChange(r.Context(), sessionToken(r), r.PathValue("incidentID"), strings.TrimSpace(r.FormValue("severity")), strings.TrimSpace(r.FormValue("reason_evidence"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.redirectOperations(w, r)
}

func (s *Server) continuitySeverityDecision(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	if err := runtime.DecideSeverity(r.Context(), sessionToken(r), r.PathValue("incidentID"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.redirectOperations(w, r)
}

func (s *Server) continuityHandover(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	if err := runtime.GenerateHandover(r.Context(), sessionToken(r), r.PathValue("incidentID"), nonBlankLines(r.FormValue("next_objectives")), nonBlankLines(r.FormValue("communication_commitments"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.redirectOperations(w, r)
}

func (s *Server) continuityResource(w http.ResponseWriter, r *http.Request) {
	runtime, ok := s.continuityCapability(w)
	if !ok || !guardMutation(w, r) {
		return
	}
	input := ResourceRequestInput{RecordID: strings.TrimSpace(r.FormValue("record_id")), Category: strings.TrimSpace(r.FormValue("category")), Request: strings.TrimSpace(r.FormValue("request")), OwnerHumanID: strings.TrimSpace(r.FormValue("owner")), Priority: strings.TrimSpace(r.FormValue("priority")), Status: strings.TrimSpace(r.FormValue("status")), AuthoritySource: strings.TrimSpace(r.FormValue("authority_source")), Notes: strings.TrimSpace(r.FormValue("notes"))}
	if err := runtime.RecordResourceRequest(r.Context(), sessionToken(r), r.PathValue("incidentID"), input); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	s.redirectOperations(w, r)
}

func (s *Server) redirectOperations(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID"))+"/operations", http.StatusSeeOther)
}

func (s *Server) renderContinuity(w http.ResponseWriter, page continuityPage) {
	page.Version = s.version
	app := appshell.AppHeader{Name: "CRAB", Status: "v" + s.version}
	shellPage := appshell.Page{App: app}
	switch {
	case page.Intake != nil:
		shellPage.DocumentTitle = "First-report intake · CRAB"
		shellPage.Title = "First-report intake"
		shellPage.Summary = "Capture first reports quickly without forcing premature incident declaration."
		shellPage.Navigation = shellNavigation("", "intake")
		shellPage.App.UserLabel = page.Intake.HumanID + " · " + page.Intake.Role
	case page.Incident != nil:
		shellPage.DocumentTitle = "Operations & continuity · CRAB"
		shellPage.Title = "Operations & continuity"
		shellPage.Summary = "Scope, severity governance, handover and resource coordination for " + page.Incident.IncidentID + "."
		shellPage.Navigation = shellNavigation(page.Incident.IncidentID, "operations")
		shellPage.App.Context = page.Incident.IncidentID
		shellPage.App.Status = strings.ToUpper(page.Incident.Mode) + " · " + page.Incident.Severity
	default:
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	s.renderFragmentShell(w, continuityTemplate, page, shellPage)
}

func nonBlankLines(raw string) []string {
	values := strings.Split(strings.ReplaceAll(raw, "\r\n", "\n"), "\n")
	out := make([]string, 0, len(values))
	for _, value := range values {
		if value = strings.TrimSpace(value); value != "" {
			out = append(out, value)
		}
	}
	return out
}
