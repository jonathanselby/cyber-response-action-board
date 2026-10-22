package web

import (
	"context"
	"html/template"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type ObligationDefinitionItem struct {
	DefinitionID string
	Definition   domain.ObligationDefinition
	Maker        string
	Confirmer    string
	ConfirmedAt  string
}

type PendingObligationItem struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  string
}

type GlobalObligationsPage struct {
	Definitions []ObligationDefinitionItem
	Pending     []PendingObligationItem
}

type IncidentObligationsPage struct {
	IncidentID  string
	Compartment string
	Clocks      []domain.ObligationClockView
	Pending     []PendingObligationItem
}

type obligationsRuntime interface {
	GlobalObligations(context.Context, string) (GlobalObligationsPage, error)
	IncidentObligations(context.Context, string, string, string, time.Time) (IncidentObligationsPage, error)
	ProposeObligationDefinition(context.Context, string, domain.ObligationDefinition) error
	ProposeObligationTrigger(context.Context, string, string, string, domain.ObligationTrigger) error
	ProposeObligationChange(context.Context, string, string, string, domain.ObligationChange) error
	DecideObligation(context.Context, string, string, string, string, string, string) error
}

var globalObligationsTemplate = template.Must(template.New("global-obligations").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Obligations unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<div class="crab-dashboard-grid crab-dashboard-grid--two">
  <section class="gas-panel"><h2>Confirmed definitions</h2><div class="crab-record-list">{{range .Definitions}}<div class="crab-record"><strong>{{.Definition.Name}}</strong><span class="gas-muted">{{.DefinitionID}} · {{.Definition.Duration}} {{.Definition.Unit}} · {{.Definition.Calendar.Timezone}}</span><span>{{.Definition.AuthoritySource}}</span></div>{{else}}<div class="gas-empty">No confirmed definitions.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Propose definition</h2><form method="post" action="/obligations/definition" class="gas-form"><div class="gas-field"><label>Name</label><input name="name" required></div><div class="gas-field"><label>Trigger field</label><input name="trigger_field" required></div><div class="gas-field"><label>Duration</label><input name="duration" type="number" min="0" required></div><div class="gas-field"><label>Unit</label><select name="unit"><option value="elapsed_hours">Elapsed hours</option><option value="calendar_days">Calendar days</option><option value="business_days">Business days</option></select></div><div class="gas-field"><label>Timezone</label><input name="timezone" value="UTC" required></div><div class="gas-field"><label>Weekend days, comma separated</label><input name="weekend" value="Saturday,Sunday"></div><div class="gas-field"><label>Holidays, YYYY-MM-DD comma separated</label><input name="holidays"></div><div class="gas-field"><label>Adjustment</label><select name="adjustment"><option value="none">None</option><option value="next_business_day">Next business day</option><option value="previous_business_day">Previous business day</option></select></div><div class="gas-field"><label>Applicability condition</label><textarea name="applicability" required></textarea></div><div class="gas-field"><label>Recipient</label><input name="recipient" required></div><div class="gas-field"><label>Owner</label><input name="owner" required></div><div class="gas-field"><label>Authority/source</label><input name="authority" required></div><div class="gas-field"><label>Warning minutes, comma separated</label><input name="warnings"></div><div><button class="gas-button gas-button--primary">Propose definition</button></div></form></section>
</div>
<section class="gas-panel crab-section-gap"><h2>Pending definition decisions</h2>{{range .Pending}}<form method="post" action="/obligations/decide" class="crab-approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><div class="crab-approval__body"><strong>{{.RecordType}}</strong><span class="gas-muted">{{.RecordID}} · {{.Maker}} · {{.ProposedAt}}</span></div><div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div></form>{{else}}<div class="gas-empty">Nothing waiting.</div>{{end}}</section>
{{end}}`))

var incidentObligationsTemplate = template.Must(template.New("incident-obligations").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Clocks unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel"><form method="get" action="/incident/{{.IncidentID}}/obligations" class="gas-form crab-inline-form"><div class="gas-field"><label>Compartment</label><input name="compartment" value="{{.Compartment}}"></div><div><button class="gas-button">Open clocks</button></div></form></section>
<div class="crab-dashboard-grid crab-section-gap">
  <section class="gas-panel"><h2>Active clocks</h2><div class="crab-record-list">{{range .Clocks}}<div class="crab-record"><div class="gas-cluster"><strong>{{.Name}}</strong>{{if .Overdue}}<span class="gas-badge gas-badge--danger">Overdue</span>{{end}}{{if .Simulated}}<span class="gas-badge gas-badge--warning">Simulated</span>{{end}}</div><span class="gas-muted">{{.ClockID}} · {{.Lifecycle}} · {{.Status}} · deadline {{.Deadline}}</span><span>{{.Recipient}} · owner {{.Owner}}</span></div>{{else}}<div class="gas-empty">No clocks in this authorised scope.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Propose trigger</h2><form method="post" action="/incident/{{.IncidentID}}/obligations/trigger" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Definition ID</label><input name="definition_id" required></div><div class="gas-field"><label>Source record ID</label><input name="source_record_id" required></div><div class="gas-field"><label>Source field</label><input name="source_field" required></div><div class="gas-field"><label>Trigger time (RFC3339)</label><input name="trigger_at" required></div><div class="gas-field"><label>Applicability reason</label><textarea name="applicability_reason" required></textarea></div><div><button class="gas-button gas-button--primary">Propose clock</button></div></form></section>
  <section class="gas-panel"><h2>Propose clock change</h2><form method="post" action="/incident/{{.IncidentID}}/obligations/change" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>Clock ID</label><input name="clock_id" required></div><div class="gas-field"><label>Kind</label><select name="kind"><option value="extend">Extend</option><option value="amend">Amend</option><option value="waive">Waive</option><option value="replace">Replace</option></select></div><div class="gas-field"><label>Authority/source</label><input name="authority" required></div><div class="gas-field"><label>Reason</label><textarea name="reason" required></textarea></div><div class="gas-field"><label>New deadline (RFC3339)</label><input name="new_deadline"></div><div class="gas-field"><label>Replacement definition ID</label><input name="replacement_definition_id"></div><div><button class="gas-button gas-button--primary">Propose change</button></div></form></section>
</div>
<section class="gas-panel crab-section-gap"><h2>Pending decisions</h2>{{range .Pending}}<form method="post" action="/obligations/decide" class="crab-approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><div class="crab-approval__body"><strong>{{.RecordType}}</strong><span class="gas-muted">{{.RecordID}} · {{.Compartment}} · {{.Maker}} · {{.ProposedAt}}</span></div><div class="gas-cluster"><button class="gas-button gas-button--primary" name="outcome" value="confirmed">Confirm</button><button class="gas-button" name="outcome" value="rejected">Reject</button></div></form>{{else}}<div class="gas-empty">Nothing waiting.</div>{{end}}</section>
{{end}}`))

func init() { registerDrop2Registrar(registerObligationRoutes) }

func registerObligationRoutes(s *Server) {
	s.mux.HandleFunc("GET /obligations", s.drop2GlobalObligations)
	s.mux.HandleFunc("POST /obligations/definition", s.drop2ObligationDefinition)
	s.mux.HandleFunc("GET /incident/{incidentID}/obligations", s.drop2IncidentObligations)
	s.mux.HandleFunc("POST /incident/{incidentID}/obligations/trigger", s.drop2ObligationTrigger)
	s.mux.HandleFunc("POST /incident/{incidentID}/obligations/change", s.drop2ObligationChange)
	s.mux.HandleFunc("POST /obligations/decide", s.drop2ObligationDecision)
}

func (s *Server) drop2GlobalObligations(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	if err != nil {
		s.renderSpecialist(w, globalObligationsTemplate, drop2Page{Title: "Obligation definitions", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.GlobalObligations(r.Context(), token)
	page := drop2Page{Title: "Obligation definitions", Summary: "Governed legal and operational clock definitions used by incidents.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, globalObligationsTemplate, page)
}

func (s *Server) drop2IncidentObligations(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderSpecialist(w, incidentObligationsTemplate, drop2Page{Title: "Obligations & clocks", IncidentID: incidentID, Error: err.Error()})
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
	view, err := runtime.IncidentObligations(r.Context(), token, incidentID, compartment, time.Now().UTC())
	page := drop2Page{Title: "Obligations & clocks", IncidentID: incidentID, Summary: "Active deadlines and governed clock changes for the authorised incident scope.", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, incidentObligationsTemplate, page)
}

func parseCSVInts(value string) ([]int, error) {
	parts := splitCSV(value)
	out := make([]int, 0, len(parts))
	for _, part := range parts {
		n, err := strconv.Atoi(part)
		if err != nil || n <= 0 {
			return nil, strconv.ErrSyntax
		}
		out = append(out, n)
	}
	return out, nil
}

func (s *Server) drop2ObligationDefinition(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	duration, err := strconv.Atoi(strings.TrimSpace(r.FormValue("duration")))
	if err != nil || duration < 0 {
		http.Error(w, "duration must be zero or greater", http.StatusBadRequest)
		return
	}
	warnings, err := parseCSVInts(r.FormValue("warnings"))
	if err != nil {
		http.Error(w, "warning minutes must be positive integers", http.StatusBadRequest)
		return
	}
	def := domain.ObligationDefinition{Name: strings.TrimSpace(r.FormValue("name")), TriggerField: strings.TrimSpace(r.FormValue("trigger_field")), Duration: duration, Unit: strings.TrimSpace(r.FormValue("unit")), Calendar: domain.BusinessCalendar{Timezone: strings.TrimSpace(r.FormValue("timezone")), Weekend: splitCSV(r.FormValue("weekend")), Holidays: splitCSV(r.FormValue("holidays")), Adjustment: strings.TrimSpace(r.FormValue("adjustment"))}, ApplicabilityCondition: strings.TrimSpace(r.FormValue("applicability")), Recipient: strings.TrimSpace(r.FormValue("recipient")), Owner: strings.TrimSpace(r.FormValue("owner")), AuthoritySource: strings.TrimSpace(r.FormValue("authority")), WarningBeforeMinutes: warnings}
	if err := runtime.ProposeObligationDefinition(r.Context(), sessionToken(r), def); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/obligations", http.StatusSeeOther)
}

func (s *Server) drop2ObligationTrigger(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	triggerAt, err := parseRequiredRFC3339(r.FormValue("trigger_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	value := domain.ObligationTrigger{DefinitionID: strings.TrimSpace(r.FormValue("definition_id")), SourceRecordID: strings.TrimSpace(r.FormValue("source_record_id")), SourceField: strings.TrimSpace(r.FormValue("source_field")), TriggerAt: triggerAt, ApplicabilityReason: strings.TrimSpace(r.FormValue("applicability_reason"))}
	if err := runtime.ProposeObligationTrigger(r.Context(), sessionToken(r), incidentID, compartment, value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/obligations?compartment="+url.QueryEscape(compartment), http.StatusSeeOther)
}

func (s *Server) drop2ObligationChange(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	deadline, err := parseOptionalRFC3339(r.FormValue("new_deadline"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	value := domain.ObligationChange{ClockID: strings.TrimSpace(r.FormValue("clock_id")), Kind: strings.TrimSpace(r.FormValue("kind")), AuthoritySource: strings.TrimSpace(r.FormValue("authority")), Reason: strings.TrimSpace(r.FormValue("reason")), NewDeadline: deadline, ReplacementDefinitionID: strings.TrimSpace(r.FormValue("replacement_definition_id"))}
	if err := runtime.ProposeObligationChange(r.Context(), sessionToken(r), incidentID, compartment, value); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/obligations?compartment="+url.QueryEscape(compartment), http.StatusSeeOther)
}

func (s *Server) drop2ObligationDecision(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	scopeID := strings.TrimSpace(r.FormValue("scope_id"))
	if err := runtime.DecideObligation(r.Context(), sessionToken(r), scopeID, r.FormValue("record_type"), r.FormValue("record_id"), r.FormValue("version"), r.FormValue("outcome")); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	if scopeID == "" {
		http.Redirect(w, r, "/obligations", http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(scopeID)+"/obligations", http.StatusSeeOther)
}
