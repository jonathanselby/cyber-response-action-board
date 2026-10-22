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

var communicationsTemplate = template.Must(template.New("communications").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Communications unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel"><form method="get" action="/incident/{{.IncidentID}}/communications" class="gas-form crab-inline-form"><div class="gas-field"><label>Compartment</label><input name="compartment" value="{{.Compartment}}"></div><div><button class="gas-button">Open</button></div></form></section>
<div class="crab-dashboard-grid crab-section-gap">
  <section class="gas-panel"><h2>Issued communications</h2><div class="crab-record-list">{{range .Issued}}<div class="crab-record"><strong>{{.Issue.Subject}}</strong><span class="gas-muted">{{.RecordID}} · {{.Issue.Kind}} · {{.Compartment}} · issued {{.IssuedAt}}</span><span>{{.Issue.Body}}</span></div>{{else}}<div class="gas-empty">No issued communications.</div>{{end}}</div><hr><form method="post" action="/incident/{{.IncidentID}}/communications/notify" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Issued communication ID</label><input name="communication_id" required></div><div class="gas-field"><label>Channels, comma separated</label><input name="channels"></div><div><button class="gas-button">Attempt notification</button></div></form></section>
  <section class="gas-panel"><h2>Draft communication</h2><form method="post" action="/incident/{{.IncidentID}}/communications/draft" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Kind</label><select name="kind"><option value="situation_report">Situation report</option><option value="executive_update">Executive update</option><option value="staff_communication">Staff communication</option><option value="holding_statement">Holding statement</option><option value="regulator_notification">Regulator notification</option><option value="individual_notice">Individual notice</option><option value="closure_pir">Closure / PIR</option></select></div><div class="gas-field"><label>Subject</label><input name="subject" required></div><div class="gas-field"><label>Body</label><textarea name="body" required></textarea></div><div class="gas-field"><label>Audience, comma separated</label><input name="audience" required></div><div class="gas-field"><label>Channels, comma separated</label><input name="channels"></div><div class="gas-field"><label>Source refs, comma separated</label><input name="source_refs"></div><div class="gas-field"><label>Related record IDs, comma separated</label><input name="related_record_ids"></div><div><button class="gas-button gas-button--primary">Record draft</button></div></form><div class="crab-record-list crab-section-gap">{{range .Drafts}}<div class="crab-record"><strong>{{.Draft.Subject}}</strong><span class="gas-muted">{{.RecordID}} · {{.Draft.Kind}} · {{.Maker}}</span></div>{{end}}</div></section>
  <section class="gas-panel"><h2>Issue approval</h2><form method="post" action="/incident/{{.IncidentID}}/communications/issue" class="gas-form"><div class="gas-field"><label>Draft record ID</label><input name="draft_id" required></div><div><button class="gas-button gas-button--primary">Propose issue</button></div></form><h3>Pending approvals</h3>{{range .Pending}}<form method="post" action="/incident/{{$.IncidentID}}/communications/decide" class="crab-approval"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><div class="crab-approval__body"><strong>{{.RecordType}}</strong><span class="gas-muted">{{.RecordID}} · {{.Maker}} · {{.Compartment}}</span></div><div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div></form>{{else}}<div class="gas-empty">Nothing waiting.</div>{{end}}</section>
</div>
<div class="crab-dashboard-grid crab-section-gap">
  <section class="gas-panel"><h2>Incoming enquiry</h2><form method="post" action="/incident/{{.IncidentID}}/communications/enquiry" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Source</label><input name="source" required></div><div class="gas-field"><label>Contact</label><input name="contact"></div><div class="gas-field"><label>Subject</label><input name="subject" required></div><div class="gas-field"><label>Message</label><textarea name="message" required></textarea></div><div class="gas-field"><label>Received at (RFC3339)</label><input name="received_at" required></div><div class="gas-field"><label>Assigned to</label><input name="assigned_to" required></div><div class="gas-field"><label>Related record IDs, comma separated</label><input name="related_record_ids"></div><div><button class="gas-button gas-button--primary">Record enquiry</button></div></form><div class="crab-record-list crab-section-gap">{{range .Enquiries}}<div class="crab-record"><strong>{{.Enquiry.Subject}}</strong><span class="gas-muted">{{.Enquiry.Source}} · {{.Enquiry.ReceivedAt}} · owner {{.Enquiry.AssignedToHumanID}}</span><span>{{.Enquiry.Message}}</span></div>{{end}}</div></section>
  <section class="gas-panel"><h2>Communication cadence</h2><form method="post" action="/incident/{{.IncidentID}}/communications/cadence" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Name</label><input name="name" required></div><div class="gas-field"><label>Interval minutes</label><input type="number" name="interval_minutes" min="1" required></div><div class="gas-field"><label>First due at (RFC3339)</label><input name="first_due_at" required></div><div class="gas-field"><label>Owner</label><input name="owner" required></div><div class="gas-field"><label>Audience, comma separated</label><input name="audience" required></div><div class="gas-field"><label>Channels, comma separated</label><input name="channels" required></div><div><button class="gas-button gas-button--primary">Propose cadence</button></div></form><div class="crab-record-list crab-section-gap">{{range .Cadences}}<div class="crab-record"><strong>{{.Cadence.Name}}</strong><span class="gas-muted">{{.CadenceID}} · every {{.Cadence.IntervalMinutes}} min · next {{.NextDueAt}}{{if .Simulated}} · simulated{{end}}</span></div>{{end}}</div></section>
  <section class="gas-panel"><h2>Generate due situation report</h2><form method="post" action="/incident/{{.IncidentID}}/communications/sitrep" class="gas-form"><div class="gas-field"><label>Cadence ID</label><input name="cadence_id" required></div><div><button class="gas-button gas-button--primary">Generate draft if due</button></div></form></section>
</div>
{{end}}`))

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
		s.renderSpecialist(w, communicationsTemplate, drop2Page{Title: "Communications & enquiries", IncidentID: incidentID, Error: err.Error()})
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
	page := drop2Page{Title: "Communications & enquiries", IncidentID: incidentID, Summary: "Drafts, approvals, issued communications, incoming enquiries and reporting cadence.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, communicationsTemplate, page)
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
