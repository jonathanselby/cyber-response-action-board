package web

import (
	"bytes"
	"context"
	"errors"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"

	appshell "github.com/jonathanselby/go-app-shell/shell"
)

const sessionCookie = "crab_session"

type BootstrapInput struct {
	PreparedBy        string
	PreparedPassword  string
	ConfirmedBy       string
	ConfirmedPassword string
}

type IncidentLink struct {
	IncidentID string
	Mode       string
	Title      string
	Severity   string
}

type FactView struct{ Statement, Source string }
type ActionView struct{ RecordID, Title, Owner, Status, Priority string }
type PendingView struct{ RecordType, RecordID, Version, Maker string }
type TimelineView struct{ Summary, RecordedAt, Governance string }

type IncidentView struct {
	IncidentID string
	Mode       string
	Title      string
	Summary    string
	Severity   string
	Facts      []FactView
	Actions    []ActionView
	Pending    []PendingView
	Timeline   []TimelineView
}

type HomeView struct {
	HumanID             string
	Role                string
	CanDeclare          bool
	Incidents           []IncidentLink
	PendingDeclarations []PendingDeclaration
}

type PendingDeclaration struct {
	IncidentID string
	RecordID   string
	Version    string
	Maker      string
	Title      string
	Summary    string
	Severity   string
	Mode       string
}

type Runtime interface {
	Bootstrapped(context.Context) (bool, error)
	Bootstrap(context.Context, BootstrapInput) error
	Authenticate(context.Context, string, string) (string, error)
	Logout(context.Context, string) error
	Home(context.Context, string) (HomeView, bool, error)
	Incident(context.Context, string, string) (IncidentView, bool, error)
	DeclareIncident(context.Context, string, string, string, string, string, string) error
	RecordFact(context.Context, string, string, string, string) error
	ProposeAction(context.Context, string, string, string, string, string, string) error
	Decide(context.Context, string, string, string, string, string, string) error
}

var RuntimeFactory func() (Runtime, error)

var setupFragment = template.Must(template.New("setup").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Setup refused</strong><span>{{.Error}}</span></div>{{end}}
<section class="gas-panel crab-auth-panel">
  <div class="gas-stack">
    <div><span class="gas-badge gas-badge--info">First run</span></div>
    <h2>Two-person bootstrap</h2>
    <p>Create both initial named accounts together. The preparer and confirmer must be different people. Bootstrap is available only from the local host.</p>
    <form method="post" action="/setup" class="gas-form">
      <div class="gas-field"><label for="prepared_by">Preparer ID</label><input id="prepared_by" name="prepared_by" autocomplete="username" required></div>
      <div class="gas-field"><label for="prepared_password">Preparer password</label><input id="prepared_password" type="password" name="prepared_password" minlength="12" autocomplete="new-password" required><span class="gas-help">At least 12 characters.</span></div>
      <div class="gas-field"><label for="confirmed_by">Confirmer ID</label><input id="confirmed_by" name="confirmed_by" autocomplete="username" required></div>
      <div class="gas-field"><label for="confirmed_password">Confirmer password</label><input id="confirmed_password" type="password" name="confirmed_password" minlength="12" autocomplete="new-password" required><span class="gas-help">Must belong to a different named human.</span></div>
      <div><button class="gas-button gas-button--primary">Confirm bootstrap</button></div>
    </form>
  </div>
</section>`))

var loginFragment = template.Must(template.New("login").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Sign in refused</strong><span>{{.Error}}</span></div>{{end}}
<section class="gas-panel crab-auth-panel">
  <div class="gas-stack">
    <h2>Sign in</h2>
    <p class="gas-muted">Use a named CRAB identity. Access to incidents and restricted compartments remains separately governed.</p>
    <form method="post" action="/login" class="gas-form">
      <div class="gas-field"><label for="human_id">Human ID</label><input id="human_id" name="human_id" autocomplete="username" required></div>
      <div class="gas-field"><label for="password">Password</label><input id="password" type="password" name="password" autocomplete="current-password" required></div>
      <div><button class="gas-button gas-button--primary">Sign in</button></div>
    </form>
  </div>
</section>`))

var homeFragment = template.Must(template.New("home").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Request refused</strong><span>{{.Error}}</span></div>{{end}}
<div class="crab-home-layout">
  <section class="gas-panel">
    <div class="crab-section-heading"><div><h2>Assigned incidents</h2><p class="gas-muted">Only incidents available to your current CRAB identity are shown.</p></div><a class="gas-button" href="/intake">First-report intake</a></div>
    {{if .Home.Incidents}}
    <ul class="gas-list crab-incident-list">
      {{range .Home.Incidents}}<li><a href="/incident/{{.IncidentID}}"><span><strong>{{.Title}}</strong><small>{{.IncidentID}} · {{.Mode}}</small></span><span class="gas-badge">{{.Severity}}</span></a></li>{{end}}
    </ul>
    {{else}}<div class="gas-empty">No incident assignments.</div>{{end}}
  </section>

  <div class="gas-stack">
    {{if .Home.CanDeclare}}
    <section class="gas-panel">
      <div class="gas-stack">
        <div><span class="gas-badge gas-badge--warning">Governed change</span></div>
        <div><h2>Declare incident</h2><p class="gas-muted">Declaration establishes governed incident state only after a different named confirmer approves it.</p></div>
        <form method="post" action="/incident" class="gas-form">
          <div class="gas-field"><label for="incident_id">Incident ID</label><input id="incident_id" name="incident_id" required></div>
          <div class="gas-field"><label for="mode">Mode</label><select id="mode" name="mode"><option value="live">Live</option><option value="exercise">Exercise</option></select></div>
          <div class="gas-field"><label for="title">Title</label><input id="title" name="title" required></div>
          <div class="gas-field"><label for="summary">Situation summary</label><textarea id="summary" name="summary" required></textarea></div>
          <div class="gas-field"><label for="severity">Severity</label><input id="severity" name="severity" required><span class="gas-help">This field is replaced by the governed organisation risk model in the next refactor chunk; it is not a release-ready control.</span></div>
          <div><button class="gas-button gas-button--primary">Propose declaration</button></div>
        </form>
      </div>
    </section>
    <section class="gas-panel">
      <h2>Pending declarations</h2>
      {{range .Home.PendingDeclarations}}
      <form method="post" action="/declaration/decide" class="crab-approval">
        <input type="hidden" name="incident_id" value="{{.IncidentID}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}">
        <div class="crab-approval__body"><strong>{{.Title}}</strong><span class="gas-muted">{{.IncidentID}} · proposed by {{.Maker}}</span><p>{{.Summary}}</p><div class="gas-cluster"><span class="gas-badge">{{.Mode}}</span><span class="gas-badge gas-badge--warning">{{.Severity}}</span></div></div>
        <div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div>
      </form>
      {{else}}<div class="gas-empty">No declarations waiting.</div>{{end}}
    </section>
    {{else}}
    <section class="gas-panel"><h2>Incident declaration</h2><p class="gas-muted">Your current role cannot propose a new incident declaration.</p></section>
    {{end}}
  </div>
</div>`))

var incidentFragment = template.Must(template.New("incident").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Request refused</strong><span>{{.Error}}</span></div>{{end}}
<section class="gas-panel crab-operating-context">
  <div class="crab-operating-context__summary"><div class="gas-cluster"><span class="gas-badge{{if eq .Incident.Mode "exercise"}} gas-badge--warning{{end}}">{{.Incident.Mode}}</span><span class="gas-badge gas-badge--warning">Severity {{.Incident.Severity}}</span></div><p>{{.Incident.Summary}}</p></div>
</section>

<div class="crab-board-grid">
  <section class="gas-panel">
    <div class="crab-section-heading"><h2>Known facts</h2><a href="#record-fact">Record fact</a></div>
    {{range .Incident.Facts}}<div class="crab-record"><strong>{{.Statement}}</strong>{{if .Source}}<span class="gas-muted">Source: {{.Source}}</span>{{end}}</div>{{else}}<div class="gas-empty">No facts recorded.</div>{{end}}
  </section>
  <section class="gas-panel">
    <div class="crab-section-heading"><h2>Live actions</h2><a href="#propose-action">Propose action</a></div>
    {{range .Incident.Actions}}<div class="crab-record"><strong>{{.Title}}</strong><span class="gas-muted">{{.Owner}} · {{.Status}} · {{.Priority}}</span></div>{{else}}<div class="gas-empty">No confirmed actions.</div>{{end}}
  </section>
</div>

<section class="gas-panel">
  <div class="crab-section-heading"><div><h2>Pending decisions</h2><p class="gas-muted">A different named human must confirm or reject the exact proposed version.</p></div></div>
  {{range .Incident.Pending}}
  <form method="post" action="/incident/{{$.Incident.IncidentID}}/decide" class="crab-approval">
    <input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}">
    <div class="crab-approval__body"><strong>{{.RecordType}}</strong><span class="gas-muted">Proposed by {{.Maker}} · version {{.Version}}</span></div>
    <div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div>
  </form>
  {{else}}<div class="gas-empty">Nothing waiting for confirmation.</div>{{end}}
</section>

<div class="crab-board-grid crab-no-print">
  <section id="record-fact" class="gas-panel">
    <h2>Record fact</h2><p class="gas-muted">Preserves an observation immediately; it does not silently change governed state.</p>
    <form method="post" action="/incident/{{.Incident.IncidentID}}/fact" class="gas-form"><div class="gas-field"><label for="fact_statement">Statement</label><textarea id="fact_statement" name="statement" required></textarea></div><div class="gas-field"><label for="fact_source">Source</label><input id="fact_source" name="source" required></div><div><button class="gas-button gas-button--primary">Record fact</button></div></form>
  </section>
  <section id="propose-action" class="gas-panel">
    <h2>Propose action</h2><p class="gas-muted">Creates a governed action proposal for separate confirmation.</p>
    <form method="post" action="/incident/{{.Incident.IncidentID}}/action" class="gas-form"><div class="gas-field"><label for="action_title">Action</label><input id="action_title" name="title" required></div><div class="gas-field"><label for="action_owner">Owner</label><input id="action_owner" name="owner" required></div><div class="gas-field"><label for="action_priority">Priority</label><input id="action_priority" name="priority" value="P1" required></div><div class="gas-field"><label for="action_source">Source reference</label><input id="action_source" name="source_ref" required></div><div><button class="gas-button gas-button--primary">Propose action</button></div></form>
  </section>
</div>

<section class="gas-panel">
  <div class="crab-section-heading"><h2>Recent history</h2><span class="gas-muted">Immutable event trail</span></div>
  {{range .Incident.Timeline}}<div class="crab-timeline-row"><strong>{{.Summary}}</strong><span class="gas-muted">{{.RecordedAt}} · {{.Governance}}</span></div>{{else}}<div class="gas-empty">No timeline entries.</div>{{end}}
</section>`))

type operatorPage struct {
	Version       string
	Error         string
	Setup         bool
	Authenticated bool
	Home          *HomeView
	Incident      *IncidentView
}

func (s *Server) registerOperatorRoutes() {
	s.mux.HandleFunc("GET /", s.operatorHome)
	s.mux.HandleFunc("POST /setup", s.operatorSetup)
	s.mux.HandleFunc("POST /login", s.operatorLogin)
	s.mux.HandleFunc("POST /logout", s.operatorLogout)
	s.mux.HandleFunc("POST /incident", s.operatorDeclare)
	s.mux.HandleFunc("POST /declaration/decide", s.operatorDeclarationDecision)
	s.mux.HandleFunc("GET /incident/{incidentID}", s.operatorIncident)
	s.mux.HandleFunc("POST /incident/{incidentID}/fact", s.operatorFact)
	s.mux.HandleFunc("POST /incident/{incidentID}/action", s.operatorAction)
	s.mux.HandleFunc("POST /incident/{incidentID}/decide", s.operatorDecision)
	s.registerContinuityRoutes()
}

func (s *Server) operatorHome(w http.ResponseWriter, r *http.Request) {
	ready, err := s.runtime.Bootstrapped(r.Context())
	if err != nil {
		s.renderOperator(w, operatorPage{Error: err.Error()})
		return
	}
	if !ready {
		s.renderOperator(w, operatorPage{Setup: true})
		return
	}
	token := sessionToken(r)
	if token == "" {
		s.renderOperator(w, operatorPage{})
		return
	}
	home, ok, err := s.runtime.Home(r.Context(), token)
	if err != nil {
		s.renderOperator(w, operatorPage{Error: err.Error()})
		return
	}
	if !ok {
		s.clearSession(w)
		s.renderOperator(w, operatorPage{})
		return
	}
	s.renderOperator(w, operatorPage{Authenticated: true, Home: &home})
}

func (s *Server) operatorSetup(w http.ResponseWriter, r *http.Request) {
	if !loopbackRequest(r) || !sameOrigin(r) {
		http.Error(w, "setup refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	input := BootstrapInput{PreparedBy: strings.TrimSpace(r.FormValue("prepared_by")), PreparedPassword: r.FormValue("prepared_password"), ConfirmedBy: strings.TrimSpace(r.FormValue("confirmed_by")), ConfirmedPassword: r.FormValue("confirmed_password")}
	if input.PreparedBy == input.ConfirmedBy || len(input.PreparedPassword) < 12 || len(input.ConfirmedPassword) < 12 {
		s.renderOperator(w, operatorPage{Setup: true, Error: "two distinct humans and passwords of at least 12 characters are required"})
		return
	}
	if err := s.runtime.Bootstrap(r.Context(), input); err != nil {
		s.renderOperator(w, operatorPage{Setup: true, Error: err.Error()})
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "login refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	token, err := s.runtime.Authenticate(r.Context(), strings.TrimSpace(r.FormValue("human_id")), r.FormValue("password"))
	if err != nil {
		s.renderOperator(w, operatorPage{Error: "authentication failed"})
		return
	}
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorLogout(w http.ResponseWriter, r *http.Request) {
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

func (s *Server) operatorIncident(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token == "" {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	incident, ok, err := s.runtime.Incident(r.Context(), token, r.PathValue("incidentID"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if !ok {
		http.NotFound(w, r)
		return
	}
	s.renderOperator(w, operatorPage{Authenticated: true, Incident: &incident})
}

func (s *Server) operatorDeclare(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	if err := s.runtime.DeclareIncident(r.Context(), sessionToken(r), strings.TrimSpace(r.FormValue("incident_id")), r.FormValue("mode"), strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("summary")), strings.TrimSpace(r.FormValue("severity"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorDeclarationDecision(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	if err := s.runtime.Decide(r.Context(), sessionToken(r), r.FormValue("incident_id"), "incident.declaration", r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorFact(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	if err := s.runtime.RecordFact(r.Context(), sessionToken(r), r.PathValue("incidentID"), strings.TrimSpace(r.FormValue("statement")), strings.TrimSpace(r.FormValue("source"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) operatorAction(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	if err := s.runtime.ProposeAction(r.Context(), sessionToken(r), r.PathValue("incidentID"), strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("owner")), strings.TrimSpace(r.FormValue("priority")), strings.TrimSpace(r.FormValue("source_ref"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) operatorDecision(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	if err := s.runtime.Decide(r.Context(), sessionToken(r), r.PathValue("incidentID"), r.FormValue("record_type"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) renderOperator(w http.ResponseWriter, page operatorPage) {
	page.Version = s.version
	base := appshell.Page{DocumentTitle: "CRAB", App: appshell.AppHeader{Name: "CRAB", Status: "v" + s.version}}
	var fragment template.HTML
	var err error
	switch {
	case page.Setup:
		base.Title = "Initial setup"
		base.Summary = "Establish the two named human identities required to initialise CRAB without creating incident-content visibility."
		fragment, err = executeOperatorFragment(setupFragment, page)
	case !page.Authenticated:
		base.Title = "Sign in"
		base.Summary = "Named authentication first; incident and compartment authority are evaluated separately."
		fragment, err = executeOperatorFragment(loginFragment, page)
	case page.Incident != nil:
		base.DocumentTitle = page.Incident.Title + " · CRAB"
		base.App.Context = page.Incident.IncidentID
		base.App.Status = strings.ToUpper(page.Incident.Mode) + " · " + page.Incident.Severity
		base.Title = page.Incident.Title
		base.Summary = "Current authorised operating picture for " + page.Incident.IncidentID + "."
		base.Navigation = shellNavigation(page.Incident.IncidentID, "board")
		base.Actions = template.HTML(`<a class="gas-button gas-button--primary" href="/incident/` + url.PathEscape(page.Incident.IncidentID) + `/operations">Operations</a>`)
		fragment, err = executeOperatorFragment(incidentFragment, page)
	case page.Home != nil:
		base.Title = "Incidents"
		base.Summary = "Assigned incident work, first-report intake and governed declaration."
		base.App.UserLabel = page.Home.HumanID + " · " + page.Home.Role
		base.Navigation = shellNavigation("", "incidents")
		base.Actions = template.HTML(`<form method="post" action="/logout"><button class="gas-button">Sign out</button></form>`)
		fragment, err = executeOperatorFragment(homeFragment, page)
	default:
		err = errors.New("operator page has no renderable state")
	}
	if err != nil {
		http.Error(w, "render failed", http.StatusInternalServerError)
		return
	}
	base.Workspace = appshell.Workspace{Mode: appshell.WorkspaceSingle, Content: fragment}
	s.renderShell(w, base)
}

func executeOperatorFragment(tmpl *template.Template, page operatorPage) (template.HTML, error) {
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, page); err != nil {
		return "", err
	}
	return template.HTML(buf.String()), nil
}

func (s *Server) clearSession(w http.ResponseWriter) {
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
}

func sessionToken(r *http.Request) string {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil {
		return ""
	}
	return cookie.Value
}

func guardMutation(w http.ResponseWriter, r *http.Request) bool {
	if sessionToken(r) == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return false
	}
	if !sameOrigin(r) {
		http.Error(w, "request origin refused", http.StatusForbidden)
		return false
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return false
	}
	return true
}

func parseSmallForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil {
		return errors.New("invalid form")
	}
	return nil
}

func sameOrigin(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		return loopbackRequest(r)
	}
	u, err := url.Parse(origin)
	if err != nil {
		return false
	}
	if strings.EqualFold(u.Host, r.Host) {
		return true
	}
	return equivalentLoopbackHosts(u.Host, r.Host)
}

func equivalentLoopbackHosts(originHost, requestHost string) bool {
	originName, originPort, err := net.SplitHostPort(originHost)
	if err != nil {
		return false
	}
	requestName, requestPort, err := net.SplitHostPort(requestHost)
	if err != nil || originPort != requestPort {
		return false
	}
	return loopbackHost(originName) && loopbackHost(requestName)
}

func loopbackHost(host string) bool {
	host = strings.TrimSuffix(strings.TrimSpace(host), ".")
	if strings.EqualFold(host, "localhost") {
		return true
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}

func loopbackRequest(r *http.Request) bool {
	if r.RemoteAddr == "" {
		return true
	}
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil {
		return false
	}
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
