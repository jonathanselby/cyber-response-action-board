package web

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type AccessDesignationView struct {
	HumanID     string
	IncidentID  string
	Compartment string
}

type AccessMembershipView struct {
	HumanID     string
	IncidentID  string
	Compartment string
	EffectiveAt string
	ExpiresAt   string
	Reason      string
	Status      string
	Implicit    bool
}

type AccessPendingView struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type AccessAdministrationPage struct {
	Sponsors               []AccessDesignationView
	Controllers            []AccessDesignationView
	IncidentMemberships    []AccessMembershipView
	CompartmentMemberships []AccessMembershipView
	Pending                []AccessPendingView
}

type ReadinessCheckView struct {
	Name    string
	Status  string
	Message string
}

type ReadinessPage struct {
	Ready  bool
	Checks []ReadinessCheckView
}

type accessReadinessRuntime interface {
	AccessAdministration(context.Context, string, time.Time) (AccessAdministrationPage, error)
	IncidentAccess(context.Context, string, string, time.Time) (AccessAdministrationPage, error)
	ProposeSponsor(context.Context, string, string, string, bool) error
	ProposeController(context.Context, string, string, string, string, bool) error
	ProposeIncidentMembership(context.Context, string, string, string, bool, *time.Time, *time.Time, string) error
	ProposeCompartmentMembership(context.Context, string, string, string, string, bool, *time.Time, *time.Time, string) error
	DecideAccess(context.Context, string, string, string, string, string, string) error
	Readiness(context.Context, string) (ReadinessPage, error)
}

var accessPageTemplate = template.Must(template.New("access").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>{{.Title}} · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>{{.Title}}</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a>{{if .IncidentID}}<span>/</span><a href="/incident/{{.IncidentID}}">{{.IncidentID}}</a>{{end}}</nav>{{if .Error}}<section class="notice notice--error"><strong>Refused</strong><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="dashboard-grid"><article><h3>Incident memberships</h3>{{range .IncidentMemberships}}<p><strong>{{.HumanID}}</strong><br><small>{{.IncidentID}} · {{.Status}}{{if .ExpiresAt}} · expires {{.ExpiresAt}}{{end}}</small></p>{{else}}<p class="muted">None visible.</p>{{end}}</article><article><h3>Sponsors</h3>{{range .Sponsors}}<p><strong>{{.HumanID}}</strong><br><small>{{.IncidentID}}</small></p>{{else}}<p class="muted">None visible.</p>{{end}}</article><article><h3>Restricted access</h3>{{range .Controllers}}<p><strong>{{.HumanID}}</strong><br><small>{{.IncidentID}} · {{.Compartment}} controller</small></p>{{end}}{{range .CompartmentMemberships}}<p><strong>{{.HumanID}}</strong><br><small>{{.Compartment}} · {{.Status}}</small></p>{{else}}<p class="muted">No restricted memberships visible.</p>{{end}}</article></section><section class="panel"><h3>Pending access changes</h3>{{range .Pending}}<form method="post" action="/access/decide" class="approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.RecordType}}</strong><small>{{.ScopeID}} · {{.Compartment}} · {{.Maker}} · {{.ProposedAt}}</small><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">Nothing waiting.</p>{{end}}</section>{{end}}{{if .IncidentID}}<section class="dashboard-grid"><article><h3>Incident member</h3><form method="post" action="/incident/{{.IncidentID}}/access/member" class="form-grid"><label>Human ID<input name="human_id" required></label><label>Active<select name="active"><option value="true">Grant</option><option value="false">Revoke</option></select></label><label>Effective at (RFC3339)<input name="effective_at"></label><label>Expires at (RFC3339)<input name="expires_at"></label><label>Reason<input name="reason"></label><button>Propose</button></form></article><article><h3>Incident Sponsor</h3><form method="post" action="/incident/{{.IncidentID}}/access/sponsor" class="form-grid"><label>Human ID<input name="human_id" required></label><label>Active<select name="active"><option value="true">Designate</option><option value="false">Remove</option></select></label><button>Propose</button></form></article><article><h3>Restricted compartment</h3><form method="post" action="/incident/{{.IncidentID}}/access/controller" class="form-grid"><label>Controller human ID<input name="human_id" required></label><label>Compartment<input name="compartment" required></label><label>Active<select name="active"><option value="true">Designate</option><option value="false">Remove</option></select></label><button>Propose controller</button></form><hr><form method="post" action="/incident/{{.IncidentID}}/access/compartment-member" class="form-grid"><label>Member human ID<input name="human_id" required></label><label>Compartment<input name="compartment" required></label><label>Active<select name="active"><option value="true">Read in</option><option value="false">Read out</option></select></label><label>Effective at (RFC3339)<input name="effective_at"></label><label>Expires at (RFC3339)<input name="expires_at"></label><label>Reason<input name="reason"></label><button>Propose membership</button></form></article></section>{{end}}</main></body></html>`))

var readinessPageTemplate = template.Must(template.New("readiness").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Readiness · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Readiness</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a><span>/</span><strong>Readiness</strong></nav>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="incident-head"><div><p class="kicker">Operational confidence</p><h2>{{if .Ready}}Ready{{else}}Needs attention{{end}}</h2></div></section><section class="panel">{{range .Checks}}<p><strong>{{.Name}}</strong><br><small>{{.Status}}{{if .Message}} · {{.Message}}{{end}}</small></p>{{end}}</section>{{end}}</main></body></html>`))

func init() { registerDrop2Registrar(registerAccessReadinessRoutes) }

func registerAccessReadinessRoutes(s *Server) {
	s.mux.HandleFunc("GET /access", s.drop2AccessAdministration)
	s.mux.HandleFunc("GET /readiness", s.drop2Readiness)
	s.mux.HandleFunc("GET /incident/{incidentID}/access", s.drop2IncidentAccess)
	s.mux.HandleFunc("POST /incident/{incidentID}/access/member", s.drop2IncidentMember)
	s.mux.HandleFunc("POST /incident/{incidentID}/access/sponsor", s.drop2Sponsor)
	s.mux.HandleFunc("POST /incident/{incidentID}/access/controller", s.drop2Controller)
	s.mux.HandleFunc("POST /incident/{incidentID}/access/compartment-member", s.drop2CompartmentMember)
	s.mux.HandleFunc("POST /access/decide", s.drop2AccessDecision)
}

func (s *Server) drop2AccessAdministration(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		s.renderDrop2(w, accessPageTemplate, drop2Page{Title: "Access administration", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.AccessAdministration(r.Context(), token, time.Now().UTC())
	page := drop2Page{Title: "Access administration", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, accessPageTemplate, page)
}

func (s *Server) drop2IncidentAccess(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderDrop2(w, accessPageTemplate, drop2Page{Title: "Incident access", IncidentID: incidentID, Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.IncidentAccess(r.Context(), token, incidentID, time.Now().UTC())
	page := drop2Page{Title: "Incident access", IncidentID: incidentID, Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, accessPageTemplate, page)
}

func (s *Server) drop2Readiness(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		s.renderDrop2(w, readinessPageTemplate, drop2Page{Title: "Readiness", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.Readiness(r.Context(), token)
	page := drop2Page{Title: "Readiness", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, readinessPageTemplate, page)
}

func (s *Server) drop2IncidentMember(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	effectiveAt, err := parseOptionalRFC3339(r.FormValue("effective_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	expiresAt, err := parseOptionalRFC3339(r.FormValue("expires_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	active := r.FormValue("active") == "true"
	incidentID := r.PathValue("incidentID")
	if err := runtime.ProposeIncidentMembership(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("human_id")), active, effectiveAt, expiresAt, strings.TrimSpace(r.FormValue("reason"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/access", http.StatusSeeOther)
}

func (s *Server) drop2Sponsor(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.ProposeSponsor(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("human_id")), r.FormValue("active") == "true"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/access", http.StatusSeeOther)
}

func (s *Server) drop2Controller(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.ProposeController(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("human_id")), strings.TrimSpace(r.FormValue("compartment")), r.FormValue("active") == "true"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/access", http.StatusSeeOther)
}

func (s *Server) drop2CompartmentMember(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	effectiveAt, err := parseOptionalRFC3339(r.FormValue("effective_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	expiresAt, err := parseOptionalRFC3339(r.FormValue("expires_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.ProposeCompartmentMembership(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("human_id")), strings.TrimSpace(r.FormValue("compartment")), r.FormValue("active") == "true", effectiveAt, expiresAt, strings.TrimSpace(r.FormValue("reason"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/access", http.StatusSeeOther)
}

func (s *Server) drop2AccessDecision(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	if err := runtime.DecideAccess(r.Context(), sessionToken(r), r.FormValue("scope_id"), r.FormValue("record_type"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/access", http.StatusSeeOther)
}
