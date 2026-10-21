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

var globalControlTemplate = template.Must(template.New("global-control").Funcs(template.FuncMap{"deref": func(v *bool) bool { return v != nil && *v }}).Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Control · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Control</h1></div><div class="version">{{.Version}}</div></header><main>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="incident-head"><div><p class="kicker">Signed in</p><h2>{{.HumanID}}</h2><p>{{.Role}}</p></div>{{if .Ready}}<div class="severity"><span>Readiness</span><strong>{{if deref .Ready}}READY{{else}}ATTENTION{{end}}</strong></div>{{end}}</section><section class="dashboard-grid"><article><h3>Assigned incidents</h3>{{range .Incidents}}<p><a href="/incident/{{.IncidentID}}/control"><strong>{{.Title}}</strong></a><br><small>{{.IncidentID}} · {{.Mode}} · {{.Severity}}</small></p>{{else}}<p class="muted">No assigned incidents.</p>{{end}}</article><article><h3>System controls</h3><p><a href="/readiness">Readiness</a></p><p><a href="/access">Access administration</a></p><p><a href="/obligations">Obligation definitions</a></p><p><a href="/intake">First-report intake</a></p></article></section>{{end}}</main></body></html>`))

var incidentControlTemplate = template.Must(template.New("incident-control").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Incident control · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Incident control</h1></div><div class="version">{{.Version}}</div></header><main>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="incident-head"><div><p class="kicker">{{.Incident.Mode}}</p><h2>{{.Incident.Title}}</h2><p>{{.Incident.IncidentID}} · {{.Incident.Summary}}</p></div><div class="severity"><span>Severity</span><strong>{{.Incident.Severity}}</strong></div></section><section class="dashboard-grid"><article><h3>Access</h3><p><strong>{{.AccessPending}}</strong> pending access changes</p><p><a href="/incident/{{.Incident.IncidentID}}/access">Open incident access</a></p></article><article><h3>Evidence</h3><p><strong>{{.EvidenceItems}}</strong> GENERAL evidence items</p><p><a href="/incident/{{.Incident.IncidentID}}/evidence">Open evidence register</a></p></article><article><h3>Obligations</h3><p><strong>{{.ActiveClocks}}</strong> active clocks · <strong>{{.OverdueClocks}}</strong> overdue</p>{{if .NextDeadline}}<p><small>Next deadline {{.NextDeadline}}</small></p>{{end}}<p><a href="/incident/{{.Incident.IncidentID}}/obligations">Open clocks</a></p></article></section><section class="dashboard-grid"><article><h3>Communications</h3><p><strong>{{.CommunicationDrafts}}</strong> drafts · <strong>{{.IssuedCommunications}}</strong> issued</p><p><strong>{{.IncomingEnquiries}}</strong> enquiries · <strong>{{.PendingCommunications}}</strong> pending approvals</p><p><a href="/incident/{{.Incident.IncidentID}}/communications">Open communications</a></p></article><article><h3>Continuity</h3><p><a href="/incident/{{.Incident.IncidentID}}/operations">Open operations, handover and continuity</a></p><p><a href="/incident/{{.Incident.IncidentID}}">Open common operating picture</a></p></article></section>{{end}}</main></body></html>`))

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
		s.renderDrop2(w, globalControlTemplate, page)
		return
	}
	view := GlobalControlPage{HumanID: home.HumanID, Role: home.Role, Incidents: home.Incidents}
	if runtime, ok := any(s.runtime).(accessReadinessRuntime); ok {
		if readiness, err := runtime.Readiness(r.Context(), token); err == nil {
			ready := readiness.Ready
			view.Ready = &ready
		}
	}
	s.renderDrop2(w, globalControlTemplate, drop2Page{Title: "Control", Data: view})
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
		s.renderDrop2(w, incidentControlTemplate, page)
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
	s.renderDrop2(w, incidentControlTemplate, drop2Page{Title: "Incident control", IncidentID: incidentID, Data: view})
}
