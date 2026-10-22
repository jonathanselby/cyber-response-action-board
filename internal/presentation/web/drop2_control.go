package web

import (
	"html/template"
	"net/http"
	"time"
)

type GlobalControlPage struct {
	HumanID   string
	Role      string
	Incidents []IncidentLink
	Ready     *bool
}

type IncidentControlPage struct {
	Incident              IncidentView
	AccessPending         int
	EvidenceItems         int
	ActiveClocks          int
	OverdueClocks         int
	CommunicationDrafts   int
	IssuedCommunications  int
	IncomingEnquiries     int
	PendingCommunications int
	NextDeadline          string
}

var globalControlTemplate = template.Must(template.New("global-control").Funcs(template.FuncMap{"deref": func(v *bool) bool { return v != nil && *v }}).Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Control unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel crab-toolbar"><div><span class="gas-badge gas-badge--info">Signed in</span><h2>{{.HumanID}}</h2><p class="gas-muted">{{.Role}}</p></div>{{if .Ready}}<div class="crab-status"><span>Readiness</span><strong>{{if deref .Ready}}READY{{else}}ATTENTION{{end}}</strong></div>{{end}}</section>
<div class="crab-dashboard-grid crab-dashboard-grid--two crab-section-gap">
  <section class="gas-panel"><h2>Assigned incidents</h2><div class="crab-record-list">{{range .Incidents}}<div class="crab-record"><a href="/incident/{{.IncidentID}}/control"><strong>{{.Title}}</strong></a><span class="gas-muted">{{.IncidentID}} · {{.Mode}} · {{.Severity}}</span></div>{{else}}<div class="gas-empty">No assigned incidents.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>System controls</h2><div class="gas-stack"><a href="/readiness">Readiness</a><a href="/access">Access administration</a><a href="/obligations">Obligation definitions</a><a href="/intake">First-report intake</a></div></section>
</div>
{{end}}`))

var incidentControlTemplate = template.Must(template.New("incident-control").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Incident control unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel crab-operating-context"><div class="crab-toolbar"><div><div class="gas-cluster"><span class="gas-badge{{if eq .Incident.Mode "exercise"}} gas-badge--warning{{end}}">{{.Incident.Mode}}</span><span class="gas-badge gas-badge--warning">Severity {{.Incident.Severity}}</span></div><h2>{{.Incident.Title}}</h2><p>{{.Incident.Summary}}</p></div></div></section>
<div class="crab-dashboard-grid crab-section-gap">
  <section class="gas-panel"><h2>Access</h2><p><strong>{{.AccessPending}}</strong> pending access changes</p><a href="/incident/{{.Incident.IncidentID}}/access">Open incident access</a></section>
  <section class="gas-panel"><h2>Evidence</h2><p><strong>{{.EvidenceItems}}</strong> GENERAL evidence items</p><a href="/incident/{{.Incident.IncidentID}}/evidence">Open evidence register</a></section>
  <section class="gas-panel"><h2>Obligations</h2><p><strong>{{.ActiveClocks}}</strong> active clocks · <strong>{{.OverdueClocks}}</strong> overdue</p>{{if .NextDeadline}}<p class="gas-muted">Next deadline {{.NextDeadline}}</p>{{end}}<a href="/incident/{{.Incident.IncidentID}}/obligations">Open clocks</a></section>
</div>
<div class="crab-dashboard-grid crab-dashboard-grid--two crab-section-gap">
  <section class="gas-panel"><h2>Communications</h2><p><strong>{{.CommunicationDrafts}}</strong> drafts · <strong>{{.IssuedCommunications}}</strong> issued</p><p><strong>{{.IncomingEnquiries}}</strong> enquiries · <strong>{{.PendingCommunications}}</strong> pending approvals</p><a href="/incident/{{.Incident.IncidentID}}/communications">Open communications</a></section>
  <section class="gas-panel"><h2>Continuity</h2><div class="gas-stack"><a href="/incident/{{.Incident.IncidentID}}/operations">Open operations, handover and continuity</a><a href="/incident/{{.Incident.IncidentID}}">Open common operating picture</a></div></section>
</div>
{{end}}`))

func init() { registerDrop2Registrar(registerControlRoutes) }

func registerControlRoutes(s *Server) {
	s.mux.HandleFunc("GET /control", s.drop2GlobalControl)
	s.mux.HandleFunc("GET /incident/{incidentID}/control", s.drop2IncidentControl)
}

func (s *Server) drop2GlobalControl(w http.ResponseWriter, r *http.Request) {
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	home, ok, err := s.runtime.Home(r.Context(), token)
	if err != nil || !ok {
		page := drop2Page{Title: "Control"}
		if err != nil {
			page.Error = err.Error()
		} else {
			page.Error = "authentication required"
		}
		s.renderSpecialist(w, globalControlTemplate, page)
		return
	}
	view := GlobalControlPage{HumanID: home.HumanID, Role: home.Role, Incidents: home.Incidents}
	if runtime, ok := any(s.runtime).(accessReadinessRuntime); ok {
		if readiness, err := runtime.Readiness(r.Context(), token); err == nil {
			ready := readiness.Ready
			view.Ready = &ready
		}
	}
	s.renderSpecialist(w, globalControlTemplate, drop2Page{Title: "Control", Summary: "Assigned incidents and the small set of organisation-level controls needed during response.", Data: view})
}

func (s *Server) drop2IncidentControl(w http.ResponseWriter, r *http.Request) {
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	incidentID := r.PathValue("incidentID")
	incident, ok, err := s.runtime.Incident(r.Context(), token, incidentID)
	if err != nil || !ok {
		page := drop2Page{Title: "Incident control", IncidentID: incidentID}
		if err != nil {
			page.Error = err.Error()
		} else {
			page.Error = "incident unavailable"
		}
		s.renderSpecialist(w, incidentControlTemplate, page)
		return
	}
	view := IncidentControlPage{Incident: incident}
	if runtime, ok := any(s.runtime).(accessReadinessRuntime); ok {
		if accessView, err := runtime.IncidentAccess(r.Context(), token, incidentID, time.Now().UTC()); err == nil {
			view.AccessPending = len(accessView.Pending)
		}
	}
	if runtime, ok := any(s.runtime).(evidenceRuntime); ok {
		if items, err := runtime.EvidenceRegister(r.Context(), token, incidentID, "GENERAL"); err == nil {
			view.EvidenceItems = len(items)
		}
	}
	if runtime, ok := any(s.runtime).(obligationsRuntime); ok {
		if obligations, err := runtime.IncidentObligations(r.Context(), token, incidentID, "GENERAL", time.Now().UTC()); err == nil {
			for _, clock := range obligations.Clocks {
				if clock.Status == "active" {
					view.ActiveClocks++
				}
				if clock.Overdue {
					view.OverdueClocks++
				}
				if clock.Status == "active" && (view.NextDeadline == "" || clock.Deadline.UTC().Format(time.RFC3339) < view.NextDeadline) {
					view.NextDeadline = clock.Deadline.UTC().Format(time.RFC3339)
				}
			}
		}
	}
	if runtime, ok := any(s.runtime).(communicationsRuntime); ok {
		if communications, err := runtime.Communications(r.Context(), token, incidentID, "GENERAL"); err == nil {
			view.CommunicationDrafts = len(communications.Drafts)
			view.IssuedCommunications = len(communications.Issued)
			view.IncomingEnquiries = len(communications.Enquiries)
			view.PendingCommunications = len(communications.Pending)
		}
	}
	s.renderSpecialist(w, incidentControlTemplate, drop2Page{Title: "Incident control", IncidentID: incidentID, Summary: "Operational control surface across access, evidence, clocks, communications and continuity.", Data: view})
}
