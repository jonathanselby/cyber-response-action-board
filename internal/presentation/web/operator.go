package web

import (
	"context"
	"errors"
	"html/template"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	operatorview "github.com/jonathanselby/cyber-response-action-board/internal/operator"
)

const sessionCookie = "crab_session"

type IncidentLink struct {
	IncidentID string
	Mode       string
	Title      string
	Severity   string
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

type BootstrapInput struct {
	PreparedBy        string
	PreparedPassword  string
	ConfirmedBy       string
	ConfirmedPassword string
}

type Runtime interface {
	Bootstrapped(context.Context) (bool, error)
	Bootstrap(context.Context, BootstrapInput) error
	Authenticate(context.Context, string, string) (string, error)
	Logout(context.Context, string) error
	Principal(context.Context, string) (operatorview.Principal, bool, error)
	ListIncidents(context.Context, operatorview.Principal) ([]IncidentLink, error)
	PendingDeclarations(context.Context, operatorview.Principal) ([]PendingDeclaration, error)
	DeclareIncident(context.Context, operatorview.Principal, string, string, string, string, string) error
	Picture(context.Context, operatorview.Principal, string, string) (domain.OperatingPicture, bool, error)
	RecordFact(context.Context, operatorview.Principal, string, string, string) error
	ProposeAction(context.Context, operatorview.Principal, string, domain.Action) error
	Decide(context.Context, operatorview.Principal, string, string, string, string, string) error
}

var RuntimeFactory func() (Runtime, error)

var operatorTemplate = template.Must(template.New("operator").Parse(`<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>CRAB</title><link rel="stylesheet" href="/static/app.css"></head>
<body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>CRAB</h1></div><div class="version">{{.Version}}</div></header><main>
{{if .Error}}<section class="notice notice--error" role="alert"><strong>Refused</strong><p>{{.Error}}</p></section>{{end}}
{{if .Setup}}
<section class="panel narrow"><p class="kicker">First run</p><h2>Two-person bootstrap</h2><form method="post" action="/setup" class="form-grid"><label>Preparer ID<input name="prepared_by" required></label><label>Preparer password<input type="password" name="prepared_password" minlength="12" required></label><label>Confirmer ID<input name="confirmed_by" required></label><label>Confirmer password<input type="password" name="confirmed_password" minlength="12" required></label><button>Confirm bootstrap</button></form></section>
{{else if not .Authenticated}}
<section class="panel narrow"><h2>Sign in</h2><form method="post" action="/login" class="form-grid"><label>Human ID<input name="human_id" required></label><label>Password<input type="password" name="password" required></label><button>Sign in</button></form></section>
{{else if .Picture}}
<nav class="crumb"><a href="/">Incidents</a><span>/</span><strong>{{.Picture.Incident.IncidentID}}</strong></nav><section class="incident-head"><div><p class="kicker">{{.Picture.Incident.Mode}}</p><h2>{{.Picture.Incident.Title}}</h2><p>{{.Picture.Incident.Summary}}</p></div><div class="severity"><span>Severity</span><strong>{{.Picture.Incident.Severity}}</strong></div></section>
<section class="dashboard-grid"><article><h3>Facts</h3>{{range .Picture.Facts}}<p><strong>{{.Fact.Statement}}</strong><br><small>{{.Fact.Source}}</small></p>{{else}}<p class="muted">No facts recorded.</p>{{end}}</article><article><h3>Actions</h3>{{range .Picture.Actions}}<p><strong>{{.Action.Title}}</strong><br><small>{{.Action.OwnerHumanID}} · {{.Action.Status}} · {{.Action.Priority}}</small></p>{{else}}<p class="muted">No confirmed actions.</p>{{end}}</article><article><h3>Pending approvals</h3>{{range .Picture.Pending}}<form method="post" action="/incident/{{$.Picture.Incident.IncidentID}}/decide" class="approval"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.RecordType}}</strong><small>{{.Maker}}</small><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">Nothing waiting.</p>{{end}}</article></section>
<section class="dashboard-grid"><article><h3>Record fact</h3><form method="post" action="/incident/{{.Picture.Incident.IncidentID}}/fact" class="form-grid"><label>Statement<textarea name="statement" required></textarea></label><label>Source<input name="source" required></label><button>Record</button></form></article><article><h3>Propose action</h3><form method="post" action="/incident/{{.Picture.Incident.IncidentID}}/action" class="form-grid"><label>Action<input name="title" required></label><label>Owner<input name="owner" required></label><label>Priority<input name="priority" value="P1" required></label><label>Source reference<input name="source_ref" required></label><button>Propose</button></form></article><article><h3>Timeline</h3>{{range .Picture.Timeline}}<p><strong>{{.Summary}}</strong><br><small>{{.RecordedAt}} · {{.Governance}}</small></p>{{else}}<p class="muted">No timeline entries.</p>{{end}}</article></section>
{{else}}
<div class="toolbar"><div><p class="kicker">Signed in</p><h2>{{.Principal.HumanID}}</h2><p>{{.Principal.Role}}</p></div><form method="post" action="/logout"><button class="secondary">Sign out</button></form></div>
{{if .CanDeclare}}<section class="dashboard-grid"><article><h3>Declare incident</h3><form method="post" action="/incident" class="form-grid"><label>Incident ID<input name="incident_id" required></label><label>Mode<select name="mode"><option value="live">Live</option><option value="exercise">Exercise</option></select></label><label>Title<input name="title" required></label><label>Summary<textarea name="summary" required></textarea></label><label>Severity<input name="severity" required></label><button>Propose declaration</button></form></article><article><h3>Pending declarations</h3>{{range .Pending}}<form method="post" action="/declaration/decide" class="approval"><input type="hidden" name="incident_id" value="{{.IncidentID}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.Title}}</strong><small>{{.IncidentID}} · {{.Maker}} · {{.Severity}}</small><p>{{.Summary}}</p><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">No declarations waiting.</p>{{end}}</article></section>{{end}}
<section class="panel"><h3>Assigned incidents</h3><div class="incident-list">{{range .Incidents}}<a class="incident-row" href="/incident/{{.IncidentID}}"><span><strong>{{.Title}}</strong><small>{{.IncidentID}} · {{.Mode}}</small></span><span>{{.Severity}}</span></a>{{else}}<p class="muted">No incident assignments.</p>{{end}}</div></section>
{{end}}</main></body></html>`))

type operatorPage struct {
	Version       string
	Error         string
	Setup         bool
	Authenticated bool
	Principal     operatorview.Principal
	Incidents     []IncidentLink
	Pending       []PendingDeclaration
	Picture       *domain.OperatingPicture
	CanDeclare    bool
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
}

func (s *Server) operatorHome(w http.ResponseWriter, r *http.Request) {
	bootstrapped, err := s.runtime.Bootstrapped(r.Context())
	if err != nil { s.renderOperator(w, operatorPage{Error: err.Error()}); return }
	if !bootstrapped { s.renderOperator(w, operatorPage{Setup: true}); return }
	principal, ok, err := s.principal(r)
	if err != nil { s.renderOperator(w, operatorPage{Error: err.Error()}); return }
	if !ok { s.renderOperator(w, operatorPage{}); return }
	incidents, err := s.runtime.ListIncidents(r.Context(), principal)
	if err != nil { s.renderOperator(w, operatorPage{Authenticated: true, Principal: principal, Error: err.Error()}); return }
	pending, err := s.runtime.PendingDeclarations(r.Context(), principal)
	if err != nil { s.renderOperator(w, operatorPage{Authenticated: true, Principal: principal, Incidents: incidents, Error: err.Error()}); return }
	s.renderOperator(w, operatorPage{Authenticated: true, Principal: principal, Incidents: incidents, Pending: pending, CanDeclare: principal.Role == operatorview.RoleMajor})
}

func (s *Server) operatorSetup(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) || !loopbackRequest(r) { http.Error(w, "setup refused", http.StatusForbidden); return }
	if err := parseSmallForm(w, r); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	input := BootstrapInput{PreparedBy: strings.TrimSpace(r.FormValue("prepared_by")), PreparedPassword: r.FormValue("prepared_password"), ConfirmedBy: strings.TrimSpace(r.FormValue("confirmed_by")), ConfirmedPassword: r.FormValue("confirmed_password")}
	if input.PreparedBy == input.ConfirmedBy || len(input.PreparedPassword) < 12 || len(input.ConfirmedPassword) < 12 { s.renderOperator(w, operatorPage{Setup: true, Error: "two distinct humans and passwords of at least 12 characters are required"}); return }
	if err := s.runtime.Bootstrap(r.Context(), input); err != nil { s.renderOperator(w, operatorPage{Setup: true, Error: err.Error()}); return }
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorLogin(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) { http.Error(w, "login refused", http.StatusForbidden); return }
	if err := parseSmallForm(w, r); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return }
	token, err := s.runtime.Authenticate(r.Context(), strings.TrimSpace(r.FormValue("human_id")), r.FormValue("password"))
	if err != nil { s.renderOperator(w, operatorPage{Error: "authentication failed"}); return }
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: token, Path: "/", HttpOnly: true, Secure: r.TLS != nil, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorLogout(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) { http.Error(w, "logout refused", http.StatusForbidden); return }
	if c, err := r.Cookie(sessionCookie); err == nil { _ = s.runtime.Logout(r.Context(), c.Value) }
	http.SetCookie(w, &http.Cookie{Name: sessionCookie, Value: "", Path: "/", MaxAge: -1, HttpOnly: true, SameSite: http.SameSiteStrictMode})
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorDeclare(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok || !guardMutation(w, r) { return }
	if err := s.runtime.DeclareIncident(r.Context(), principal, strings.TrimSpace(r.FormValue("incident_id")), r.FormValue("mode"), strings.TrimSpace(r.FormValue("title")), strings.TrimSpace(r.FormValue("summary")), strings.TrimSpace(r.FormValue("severity"))); err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorDeclarationDecision(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok || !guardMutation(w, r) { return }
	if err := s.runtime.Decide(r.Context(), principal, r.FormValue("incident_id"), domain.TypeIncidentDeclaration, r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (s *Server) operatorIncident(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok { return }
	picture, found, err := s.runtime.Picture(r.Context(), principal, r.PathValue("incidentID"), "GENERAL")
	if err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	if !found { http.NotFound(w, r); return }
	s.renderOperator(w, operatorPage{Authenticated: true, Principal: principal, Picture: &picture})
}

func (s *Server) operatorFact(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok || !guardMutation(w, r) { return }
	if err := s.runtime.RecordFact(r.Context(), principal, r.PathValue("incidentID"), strings.TrimSpace(r.FormValue("statement")), strings.TrimSpace(r.FormValue("source"))); err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) operatorAction(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok || !guardMutation(w, r) { return }
	action := domain.Action{Title: strings.TrimSpace(r.FormValue("title")), OwnerHumanID: strings.TrimSpace(r.FormValue("owner")), Status: domain.ActionStatusNotStarted, Priority: strings.TrimSpace(r.FormValue("priority")), SourceRef: strings.TrimSpace(r.FormValue("source_ref"))}
	if err := s.runtime.ProposeAction(r.Context(), principal, r.PathValue("incidentID"), action); err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) operatorDecision(w http.ResponseWriter, r *http.Request) {
	principal, ok := s.requirePrincipal(w, r); if !ok || !guardMutation(w, r) { return }
	if err := s.runtime.Decide(r.Context(), principal, r.PathValue("incidentID"), r.FormValue("record_type"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil { http.Error(w, err.Error(), http.StatusForbidden); return }
	http.Redirect(w, r, "/incident/"+url.PathEscape(r.PathValue("incidentID")), http.StatusSeeOther)
}

func (s *Server) principal(r *http.Request) (operatorview.Principal, bool, error) {
	cookie, err := r.Cookie(sessionCookie)
	if err != nil { return operatorview.Principal{}, false, nil }
	return s.runtime.Principal(r.Context(), cookie.Value)
}

func (s *Server) requirePrincipal(w http.ResponseWriter, r *http.Request) (operatorview.Principal, bool) {
	principal, ok, err := s.principal(r)
	if err != nil { http.Error(w, "session unavailable", http.StatusServiceUnavailable); return operatorview.Principal{}, false }
	if !ok { http.Redirect(w, r, "/", http.StatusSeeOther); return operatorview.Principal{}, false }
	return principal, true
}

func (s *Server) renderOperator(w http.ResponseWriter, page operatorPage) {
	page.Version = s.version
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := operatorTemplate.Execute(w, page); err != nil { http.Error(w, "render failed", http.StatusInternalServerError) }
}

func guardMutation(w http.ResponseWriter, r *http.Request) bool {
	if !sameOrigin(r) { http.Error(w, "request origin refused", http.StatusForbidden); return false }
	if err := parseSmallForm(w, r); err != nil { http.Error(w, err.Error(), http.StatusBadRequest); return false }
	return true
}

func parseSmallForm(w http.ResponseWriter, r *http.Request) error {
	r.Body = http.MaxBytesReader(w, r.Body, 64<<10)
	if err := r.ParseForm(); err != nil { return errors.New("invalid form") }
	return nil
}

func sameOrigin(r *http.Request) bool {
	origin := r.Header.Get("Origin")
	if origin == "" { return loopbackRequest(r) }
	u, err := url.Parse(origin)
	return err == nil && strings.EqualFold(u.Host, r.Host)
}

func loopbackRequest(r *http.Request) bool {
	if r.RemoteAddr == "" { return true }
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err != nil { return false }
	ip := net.ParseIP(host)
	return ip != nil && ip.IsLoopback()
}
