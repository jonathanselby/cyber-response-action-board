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

var accessPageTemplate = template.Must(template.New("access").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Access request refused</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<div class="crab-dashboard-grid">
  <section class="gas-panel"><h2>Incident memberships</h2><div class="crab-record-list">{{range .IncidentMemberships}}<div class="crab-record"><strong>{{.HumanID}}</strong><span class="gas-muted">{{.IncidentID}} · {{.Status}}{{if .ExpiresAt}} · expires {{.ExpiresAt}}{{end}}</span>{{if .Reason}}<span>{{.Reason}}</span>{{end}}</div>{{else}}<div class="gas-empty">None visible.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Sponsors</h2><div class="crab-record-list">{{range .Sponsors}}<div class="crab-record"><strong>{{.HumanID}}</strong><span class="gas-muted">{{.IncidentID}}</span></div>{{else}}<div class="gas-empty">None visible.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Restricted access</h2><div class="crab-record-list">{{range .Controllers}}<div class="crab-record"><strong>{{.HumanID}}</strong><span class="gas-muted">{{.IncidentID}} · {{.Compartment}} controller</span></div>{{end}}{{range .CompartmentMemberships}}<div class="crab-record"><strong>{{.HumanID}}</strong><span class="gas-muted">{{.Compartment}} · {{.Status}}</span>{{if .Reason}}<span>{{.Reason}}</span>{{end}}</div>{{else}}<div class="gas-empty">No restricted memberships visible.</div>{{end}}</div></section>
</div>
<section class="gas-panel crab-section-gap"><h2>Pending access changes</h2>{{range .Pending}}<form method="post" action="/access/decide" class="crab-approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><div class="crab-approval__body"><strong>{{.RecordType}}</strong><span class="gas-muted">{{.ScopeID}} · {{.Compartment}} · {{.Maker}} · {{.ProposedAt}}</span></div><div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div></form>{{else}}<div class="gas-empty">Nothing waiting.</div>{{end}}</section>
{{end}}
{{if .IncidentID}}
<div class="crab-dashboard-grid crab-section-gap">
  <section class="gas-panel"><h2>Incident member</h2><form method="post" action="/incident/{{.IncidentID}}/access/member" class="gas-form"><div class="gas-field"><label>Human ID</label><input name="human_id" required></div><div class="gas-field"><label>Access</label><select name="active"><option value="true">Grant</option><option value="false">Revoke</option></select></div><div class="gas-field"><label>Effective at (RFC3339)</label><input name="effective_at"></div><div class="gas-field"><label>Expires at (RFC3339)</label><input name="expires_at"></div><div class="gas-field"><label>Reason</label><input name="reason"></div><div><button class="gas-button gas-button--primary">Propose</button></div></form></section>
  <section class="gas-panel"><h2>Incident sponsor</h2><form method="post" action="/incident/{{.IncidentID}}/access/sponsor" class="gas-form"><div class="gas-field"><label>Human ID</label><input name="human_id" required></div><div class="gas-field"><label>Designation</label><select name="active"><option value="true">Designate</option><option value="false">Remove</option></select></div><div><button class="gas-button gas-button--primary">Propose</button></div></form></section>
  <section class="gas-panel"><h2>Restricted compartment</h2><form method="post" action="/incident/{{.IncidentID}}/access/controller" class="gas-form"><div class="gas-field"><label>Controller human ID</label><input name="human_id" required></div><div class="gas-field"><label>Compartment</label><input name="compartment" required></div><div class="gas-field"><label>Designation</label><select name="active"><option value="true">Designate</option><option value="false">Remove</option></select></div><div><button class="gas-button gas-button--primary">Propose controller</button></div></form><hr><form method="post" action="/incident/{{.IncidentID}}/access/compartment-member" class="gas-form"><div class="gas-field"><label>Member human ID</label><input name="human_id" required></div><div class="gas-field"><label>Compartment</label><input name="compartment" required></div><div class="gas-field"><label>Membership</label><select name="active"><option value="true">Read in</option><option value="false">Read out</option></select></div><div class="gas-field"><label>Effective at (RFC3339)</label><input name="effective_at"></div><div class="gas-field"><label>Expires at (RFC3339)</label><input name="expires_at"></div><div class="gas-field"><label>Reason</label><input name="reason"></div><div><button class="gas-button gas-button--primary">Propose membership</button></div></form></section>
</div>
{{end}}`))

var readinessPageTemplate = template.Must(template.New("readiness").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Readiness unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel crab-readiness-summary"><div><span class="gas-badge{{if .Ready}} gas-badge--success{{else}} gas-badge--warning{{end}}">{{if .Ready}}Ready{{else}}Needs attention{{end}}</span><p class="gas-muted">Operational confidence signal. Detailed checks are listed below.</p></div></section>
<section class="gas-panel crab-section-gap"><h2>Checks</h2><div class="crab-record-list">{{range .Checks}}<div class="crab-record"><strong>{{.Name}}</strong><span class="gas-muted">{{.Status}}{{if .Message}} · {{.Message}}{{end}}</span></div>{{else}}<div class="gas-empty">No readiness checks reported.</div>{{end}}</div></section>
{{end}}`))

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
		s.renderSpecialist(w, accessPageTemplate, drop2Page{Title: "Access administration", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.AccessAdministration(r.Context(), token, time.Now().UTC())
	page := drop2Page{Title: "Access administration", Summary: "Organisation access governance without exposing incident content by default.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, accessPageTemplate, page)
}

func (s *Server) drop2IncidentAccess(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderSpecialist(w, accessPageTemplate, drop2Page{Title: "Incident access", IncidentID: incidentID, Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.IncidentAccess(r.Context(), token, incidentID, time.Now().UTC())
	page := drop2Page{Title: "Incident access", IncidentID: incidentID, Summary: "Incident membership, sponsorship and restricted-compartment read-in.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, accessPageTemplate, page)
}

func (s *Server) drop2Readiness(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[accessReadinessRuntime](s)
	if err != nil {
		s.renderSpecialist(w, readinessPageTemplate, drop2Page{Title: "Readiness", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.Readiness(r.Context(), token)
	page := drop2Page{Title: "Readiness", Summary: "Compact operational confidence signal with detailed technical checks behind it.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, readinessPageTemplate, page)
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
