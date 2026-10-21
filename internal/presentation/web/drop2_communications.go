package web

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type PendingCommunicationItem struct {
	IncidentID  string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type CommunicationsPage struct {
	IncidentID  string
	Compartment string
	Drafts      []domain.CommunicationDraftView
	Issued      []domain.CommunicationView
	Cadences    []domain.CommunicationCadenceView
	Enquiries   []domain.IncomingEnquiryView
	Pending     []PendingCommunicationItem
}

type communicationsRuntime interface {
	Communications(context.Context, string, string, string) (CommunicationsPage, error)
	RecordCommunicationDraft(context.Context, string, string, string, domain.CommunicationDraft) error
	ProposeCommunicationIssue(context.Context, string, string, string) error
	ProposeCommunicationCadence(context.Context, string, string, string, domain.CommunicationCadence) error
	RecordIncomingEnquiry(context.Context, string, string, string, domain.IncomingEnquiry) error
	GenerateSituationReport(context.Context, string, string, string, string) error
	NotifyCommunication(context.Context, string, string, string, []string) error
	DecideCommunication(context.Context, string, string, string, string, string, string) error
}

var communicationsTemplate = template.Must(template.New("communications").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Communications · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Communications & enquiries</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a><span>/</span><a href="/incident/{{.IncidentID}}">{{.IncidentID}}</a><span>/</span><strong>Communications</strong></nav>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="panel"><form method="get" action="/incident/{{.IncidentID}}/communications" class="form-grid"><label>Compartment<input name="compartment" value="{{.Compartment}}"></label><button>Open</button></form></section><section class="dashboard-grid"><article><h3>Issued communications</h3>{{range .Issued}}<p><strong>{{.Issue.Subject}}</strong><br><small>{{.RecordID}} · {{.Issue.Kind}} · {{.Compartment}} · issued {{.IssuedAt}}</small><br>{{.Issue.Body}}</p>{{else}}<p class="muted">No issued communications.</p>{{end}}<form method="post" action="/incident/{{.IncidentID}}/communications/notify" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Issued communication ID<input name="communication_id" required></label><label>Channels, comma separated<input name="channels"></label><button>Attempt notification</button></form></article><article><h3>Draft communication</h3><form method="post" action="/incident/{{.IncidentID}}/communications/draft" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Kind<select name="kind"><option value="situation_report">Situation report</option><option value="executive_update">Executive update</option><option value="staff_communication">Staff communication</option><option value="holding_statement">Holding statement</option><option value="regulator_notification">Regulator notification</option><option value="individual_notice">Individual notice</option><option value="closure_pir">Closure / PIR</option></select></label><label>Subject<input name="subject" required></label><label>Body<textarea name="body" required></textarea></label><label>Audience, comma separated<input name="audience" required></label><label>Channels, comma separated<input name="channels"></label><label>Source refs, comma separated<input name="source_refs"></label><label>Related record IDs, comma separated<input name="related_record_ids"></label><button>Record draft</button></form>{{range .Drafts}}<p><strong>{{.Draft.Subject}}</strong><br><small>{{.RecordID}} · {{.Draft.Kind}} · {{.Maker}}</small></p>{{end}}</article><article><h3>Propose issue</h3><form method="post" action="/incident/{{.IncidentID}}/communications/issue" class="form-grid"><label>Draft record ID<input name="draft_id" required></label><button>Propose issue</button></form><h3>Pending approvals</h3>{{range .Pending}}<form method="post" action="/incident/{{$.IncidentID}}/communications/decide" class="approval"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.RecordType}}</strong><small>{{.RecordID}} · {{.Maker}} · {{.Compartment}}</small><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">Nothing waiting.</p>{{end}}</article></section><section class="dashboard-grid"><article><h3>Incoming enquiry</h3><form method="post" action="/incident/{{.IncidentID}}/communications/enquiry" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Source<input name="source" required></label><label>Contact<input name="contact"></label><label>Subject<input name="subject" required></label><label>Message<textarea name="message" required></textarea></label><label>Received at (RFC3339)<input name="received_at" required></label><label>Assigned to<input name="assigned_to" required></label><label>Related record IDs, comma separated<input name="related_record_ids"></label><button>Record enquiry</button></form>{{range .Enquiries}}<p><strong>{{.Enquiry.Subject}}</strong><br><small>{{.Enquiry.Source}} · {{.Enquiry.ReceivedAt}} · owner {{.Enquiry.AssignedToHumanID}}</small><br>{{.Enquiry.Message}}</p>{{end}}</article><article><h3>Communication cadence</h3><form method="post" action="/incident/{{.IncidentID}}/communications/cadence" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Name<input name="name" required></label><label>Interval minutes<input type="number" name="interval_minutes" min="1" required></label><label>First due at (RFC3339)<input name="first_due_at" required></label><label>Owner<input name="owner" required></label><label>Audience, comma separated<input name="audience" required></label><label>Channels, comma separated<input name="channels" required></label><button>Propose cadence</button></form>{{range .Cadences}}<p><strong>{{.Cadence.Name}}</strong><br><small>{{.CadenceID}} · every {{.Cadence.IntervalMinutes}} min · next {{.NextDueAt}}{{if .Simulated}} · simulated{{end}}</small></p>{{end}}</article><article><h3>Generate due situation report</h3><form method="post" action="/incident/{{.IncidentID}}/communications/sitrep" class="form-grid"><label>Cadence ID<input name="cadence_id" required></label><button>Generate draft if due</button></form></article></section>{{end}}</main></body></html>`))

func init() { registerDrop2Registrar(registerCommunicationRoutes) }

func registerCommunicationRoutes(s *Server) {
	s.mux.HandleFunc("GET /incident/{incidentID}/communications", s.drop2Communications)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/draft", s.drop2CommunicationDraft)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/issue", s.drop2CommunicationIssue)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/cadence", s.drop2CommunicationCadence)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/enquiry", s.drop2CommunicationEnquiry)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/sitrep", s.drop2SituationReport)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/notify", s.drop2NotifyCommunication)
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/decide", s.drop2CommunicationDecision)
}

func (s *Server) drop2Communications(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderDrop2(w, communicationsTemplate, drop2Page{Title: "Communications", IncidentID: incidentID, Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	compartment := strings.TrimSpace(r.URL.Query().Get("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	view, err := runtime.Communications(r.Context(), token, incidentID, compartment)
	page := drop2Page{Title: "Communications", IncidentID: incidentID, Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, communicationsTemplate, page)
}

func (s *Server) drop2CommunicationDraft(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	value := domain.CommunicationDraft{Kind: strings.TrimSpace(r.FormValue("kind")), Subject: strings.TrimSpace(r.FormValue("subject")), Body: strings.TrimSpace(r.FormValue("body")), Audience: splitCSV(r.FormValue("audience")), Channels: splitCSV(r.FormValue("channels")), SourceRefs: splitCSV(r.FormValue("source_refs")), RelatedRecordIDs: splitCSV(r.FormValue("related_record_ids"))}
	if err := runtime.RecordCommunicationDraft(r.Context(), sessionToken(r), incidentID, compartment, value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, compartment), http.StatusSeeOther)
}

func (s *Server) drop2CommunicationIssue(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.ProposeCommunicationIssue(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("draft_id"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, "GENERAL"), http.StatusSeeOther)
}

func (s *Server) drop2CommunicationCadence(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	interval, err := parsePositiveInt(r.FormValue("interval_minutes"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	firstDue, err := parseRequiredRFC3339(r.FormValue("first_due_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	value := domain.CommunicationCadence{Name: strings.TrimSpace(r.FormValue("name")), IntervalMinutes: interval, FirstDueAt: firstDue, OwnerHumanID: strings.TrimSpace(r.FormValue("owner")), Audience: splitCSV(r.FormValue("audience")), Channels: splitCSV(r.FormValue("channels"))}
	if err := runtime.ProposeCommunicationCadence(r.Context(), sessionToken(r), incidentID, compartment, value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, compartment), http.StatusSeeOther)
}

func (s *Server) drop2CommunicationEnquiry(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	receivedAt, err := parseRequiredRFC3339(r.FormValue("received_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	value := domain.IncomingEnquiry{Source: strings.TrimSpace(r.FormValue("source")), Contact: strings.TrimSpace(r.FormValue("contact")), Subject: strings.TrimSpace(r.FormValue("subject")), Message: strings.TrimSpace(r.FormValue("message")), ReceivedAt: receivedAt, AssignedToHumanID: strings.TrimSpace(r.FormValue("assigned_to")), RelatedRecordIDs: splitCSV(r.FormValue("related_record_ids"))}
	if err := runtime.RecordIncomingEnquiry(r.Context(), sessionToken(r), incidentID, compartment, value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, compartment), http.StatusSeeOther)
}

func (s *Server) drop2SituationReport(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.GenerateSituationReport(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("cadence_id")), "GENERAL"); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, "GENERAL"), http.StatusSeeOther)
}

func (s *Server) drop2NotifyCommunication(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	if err := runtime.NotifyCommunication(r.Context(), sessionToken(r), incidentID, strings.TrimSpace(r.FormValue("communication_id")), splitCSV(r.FormValue("channels"))); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, compartment), http.StatusSeeOther)
}

func (s *Server) drop2CommunicationDecision(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[communicationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	incidentID := r.PathValue("incidentID")
	if err := runtime.DecideCommunication(r.Context(), sessionToken(r), incidentID, r.FormValue("record_type"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, communicationURL(incidentID, "GENERAL"), http.StatusSeeOther)
}

func communicationURL(incidentID, compartment string) string {
	return "/incident/" + url.PathEscape(incidentID) + "/communications?compartment=" + url.QueryEscape(compartment)
}
