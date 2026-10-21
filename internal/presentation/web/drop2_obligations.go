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

var globalObligationsTemplate = template.Must(template.New("global-obligations").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Obligations · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Obligation definitions</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a><span>/</span><strong>Obligations</strong></nav>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="dashboard-grid"><article><h3>Confirmed definitions</h3>{{range .Definitions}}<p><strong>{{.Definition.Name}}</strong><br><small>{{.DefinitionID}} · {{.Definition.Duration}} {{.Definition.Unit}} · {{.Definition.Calendar.Timezone}} · {{.Definition.AuthoritySource}}</small></p>{{else}}<p class="muted">No confirmed definitions.</p>{{end}}</article><article><h3>Propose definition</h3><form method="post" action="/obligations/definition" class="form-grid"><label>Name<input name="name" required></label><label>Trigger field<input name="trigger_field" required></label><label>Duration<input name="duration" type="number" min="0" required></label><label>Unit<select name="unit"><option value="elapsed_hours">Elapsed hours</option><option value="calendar_days">Calendar days</option><option value="business_days">Business days</option></select></label><label>Timezone<input name="timezone" value="UTC" required></label><label>Weekend days, comma separated<input name="weekend" value="Saturday,Sunday"></label><label>Holidays, YYYY-MM-DD comma separated<input name="holidays"></label><label>Adjustment<select name="adjustment"><option value="none">None</option><option value="next_business_day">Next business day</option><option value="previous_business_day">Previous business day</option></select></label><label>Applicability condition<textarea name="applicability" required></textarea></label><label>Recipient<input name="recipient" required></label><label>Owner<input name="owner" required></label><label>Authority/source<input name="authority" required></label><label>Warning minutes, comma separated<input name="warnings"></label><button>Propose definition</button></form></article></section><section class="panel"><h3>Pending definition decisions</h3>{{range .Pending}}<form method="post" action="/obligations/decide" class="approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.RecordType}}</strong><small>{{.RecordID}} · {{.Maker}} · {{.ProposedAt}}</small><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">Nothing waiting.</p>{{end}}</section>{{end}}</main></body></html>`))

var incidentObligationsTemplate = template.Must(template.New("incident-obligations").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Clocks · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Obligations & clocks</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a><span>/</span><a href="/incident/{{.IncidentID}}">{{.IncidentID}}</a><span>/</span><strong>Obligations</strong></nav>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="panel"><form method="get" action="/incident/{{.IncidentID}}/obligations" class="form-grid"><label>Compartment<input name="compartment" value="{{.Compartment}}"></label><button>Open clocks</button></form></section><section class="dashboard-grid"><article><h3>Active clocks</h3>{{range .Clocks}}<p><strong>{{.Name}}</strong><br><small>{{.ClockID}} · {{.Lifecycle}} · {{.Status}} · deadline {{.Deadline}} {{if .Overdue}}· OVERDUE{{end}}{{if .Simulated}} · simulated{{end}}</small><br>{{.Recipient}} · owner {{.Owner}}</p>{{else}}<p class="muted">No clocks in this authorised scope.</p>{{end}}</article><article><h3>Propose trigger</h3><form method="post" action="/incident/{{.IncidentID}}/obligations/trigger" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Definition ID<input name="definition_id" required></label><label>Source record ID<input name="source_record_id" required></label><label>Source field<input name="source_field" required></label><label>Trigger time (RFC3339)<input name="trigger_at" required></label><label>Applicability reason<textarea name="applicability_reason" required></textarea></label><button>Propose clock</button></form></article><article><h3>Propose clock change</h3><form method="post" action="/incident/{{.IncidentID}}/obligations/change" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>Clock ID<input name="clock_id" required></label><label>Kind<select name="kind"><option value="extend">Extend</option><option value="amend">Amend</option><option value="waive">Waive</option><option value="replace">Replace</option></select></label><label>Authority/source<input name="authority" required></label><label>Reason<textarea name="reason" required></textarea></label><label>New deadline (RFC3339)<input name="new_deadline"></label><label>Replacement definition ID<input name="replacement_definition_id"></label><button>Propose change</button></form></article></section><section class="panel"><h3>Pending decisions</h3>{{range .Pending}}<form method="post" action="/obligations/decide" class="approval"><input type="hidden" name="scope_id" value="{{.ScopeID}}"><input type="hidden" name="record_type" value="{{.RecordType}}"><input type="hidden" name="record_id" value="{{.RecordID}}"><input type="hidden" name="version" value="{{.Version}}"><strong>{{.RecordType}}</strong><small>{{.RecordID}} · {{.Compartment}} · {{.Maker}} · {{.ProposedAt}}</small><div><button name="outcome" value="confirmed">Confirm</button><button class="secondary" name="outcome" value="rejected">Reject</button></div></form>{{else}}<p class="muted">Nothing waiting.</p>{{end}}</section>{{end}}</main></body></html>`))

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
		s.renderDrop2(w, globalObligationsTemplate, drop2Page{Title: "Obligations", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	view, err := runtime.GlobalObligations(r.Context(), token)
	page := drop2Page{Title: "Obligations", Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, globalObligationsTemplate, page)
}

func (s *Server) drop2IncidentObligations(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[obligationsRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderDrop2(w, incidentObligationsTemplate, drop2Page{Title: "Obligations", IncidentID: incidentID, Error: err.Error()})
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
	page := drop2Page{Title: "Obligations", IncidentID: incidentID, Data: view}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, incidentObligationsTemplate, page)
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
