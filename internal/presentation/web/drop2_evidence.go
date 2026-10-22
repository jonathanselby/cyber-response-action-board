package web

import (
	"context"
	"html/template"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type EvidenceRegisterPage struct {
	IncidentID  string
	Compartment string
	Items       []domain.EvidenceView
}

type EvidenceDetailPage struct {
	Item    domain.EvidenceView
	History []domain.EvidenceHandlingView
}

type evidenceRuntime interface {
	EvidenceRegister(context.Context, string, string, string) ([]domain.EvidenceView, error)
	CaptureEvidence(context.Context, string, string, string, string, string, string, string, string, *time.Time, *time.Time, string, string, string, io.Reader) error
	EvidenceDetail(context.Context, string, string) (domain.EvidenceView, error)
	EvidenceHistory(context.Context, string, string) ([]domain.EvidenceHandlingView, error)
	RecordEvidenceHandling(context.Context, string, string, string, string, string, string, *time.Time) error
	DownloadEvidence(context.Context, string, string, io.Writer) error
}

var evidenceRegisterTemplate = template.Must(template.New("evidence-register").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Evidence unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<section class="gas-panel"><form method="get" action="/incident/{{.IncidentID}}/evidence" class="gas-form crab-inline-form"><div class="gas-field"><label>Compartment</label><input name="compartment" value="{{.Compartment}}"></div><div><button class="gas-button">Open register</button></div></form></section>
<div class="crab-dashboard-grid crab-dashboard-grid--two crab-section-gap">
  <section class="gas-panel"><h2>Register</h2><div class="crab-record-list">{{range .Items}}<div class="crab-record"><a href="/evidence/{{.EvidenceID}}"><strong>{{.Evidence.OriginalFilename}}</strong></a><span class="gas-muted">{{.EvidenceID}} · {{.Compartment}} · {{.Evidence.SizeBytes}} bytes · {{.Evidence.DetectedMediaType}}</span></div>{{else}}<div class="gas-empty">No evidence in this authorised scope.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Capture evidence</h2><form method="post" enctype="multipart/form-data" action="/incident/{{.IncidentID}}/evidence" class="gas-form"><input type="hidden" name="compartment" value="{{.Compartment}}"><div class="gas-field"><label>File</label><input type="file" name="evidence" accept="image/*,audio/*,video/*,*/*" capture required></div><div class="gas-field"><label>Description</label><textarea name="description"></textarea></div><div class="gas-field"><label>Collector</label><input name="collector" required></div><div class="gas-field"><label>Source</label><input name="source" required></div><div class="gas-field"><label>Current handler</label><input name="current_handler" required></div><div class="gas-field"><label>Source location</label><input name="source_location"></div><div class="gas-field"><label>Related reference</label><input name="related_ref"></div><div class="gas-field"><label>Collected at (RFC3339)</label><input name="collected_at"></div><div class="gas-field"><label>Occurred at (RFC3339)</label><input name="occurred_at"></div><div><button class="gas-button gas-button--primary">Preserve evidence</button></div></form></section>
</div>
{{end}}`))

var evidenceDetailTemplate = template.Must(template.New("evidence-detail").Parse(`
{{if .Error}}<div class="gas-error-summary" role="alert"><strong>Evidence unavailable</strong><span>{{.Error}}</span></div>{{end}}
{{with .Data}}
<div class="crab-dashboard-grid">
  <section class="gas-panel"><h2>{{.Item.Evidence.OriginalFilename}}</h2><p>{{.Item.Evidence.Description}}</p><dl class="gas-kv"><dt>Evidence ID</dt><dd>{{.Item.EvidenceID}}</dd><dt>Compartment</dt><dd>{{.Item.Compartment}}</dd><dt>SHA-256</dt><dd class="crab-break-word">{{.Item.Evidence.SHA256}}</dd><dt>Size</dt><dd>{{.Item.Evidence.SizeBytes}} bytes</dd><dt>Media type</dt><dd>declared {{.Item.Evidence.DeclaredMediaType}} · detected {{.Item.Evidence.DetectedMediaType}}</dd><dt>Collector</dt><dd>{{.Item.Evidence.Collector}}</dd><dt>Source</dt><dd>{{.Item.Evidence.Source}}</dd><dt>Handler</dt><dd>{{.Item.Evidence.CurrentHandler}}</dd></dl><p><a class="gas-button" href="/evidence/{{.Item.EvidenceID}}/attachment">Authorised view/download</a></p></section>
  <section class="gas-panel"><h2>Handling history</h2><div class="crab-record-list">{{range .History}}<div class="crab-record"><strong>{{.Handling.Action}}</strong><span class="gas-muted">{{.Handling.Handler}} · {{.RecordedAt}}{{if .Handling.Location}} · {{.Handling.Location}}{{end}}</span>{{if .Handling.Note}}<span>{{.Handling.Note}}</span>{{end}}</div>{{else}}<div class="gas-empty">No handling records.</div>{{end}}</div></section>
  <section class="gas-panel"><h2>Record handling</h2><form method="post" action="/evidence/{{.Item.EvidenceID}}/handling" class="gas-form"><div class="gas-field"><label>Action</label><input name="action" required></div><div class="gas-field"><label>Handler</label><input name="handler" required></div><div class="gas-field"><label>Location</label><input name="location"></div><div class="gas-field"><label>Occurred at (RFC3339)</label><input name="occurred_at"></div><div class="gas-field"><label>Note</label><textarea name="note"></textarea></div><div><button class="gas-button gas-button--primary">Record</button></div></form></section>
</div>
{{end}}`))

func init() { registerDrop2Registrar(registerEvidenceRoutes) }

func registerEvidenceRoutes(s *Server) {
	s.mux.HandleFunc("GET /incident/{incidentID}/evidence", s.drop2EvidenceRegister)
	s.mux.HandleFunc("POST /incident/{incidentID}/evidence", s.drop2EvidenceCapture)
	s.mux.HandleFunc("GET /evidence/{evidenceID}", s.drop2EvidenceDetail)
	s.mux.HandleFunc("GET /evidence/{evidenceID}/attachment", s.drop2EvidenceAttachment)
	s.mux.HandleFunc("POST /evidence/{evidenceID}/handling", s.drop2EvidenceHandling)
}

func (s *Server) drop2EvidenceRegister(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[evidenceRuntime](s)
	incidentID := r.PathValue("incidentID")
	if err != nil {
		s.renderSpecialist(w, evidenceRegisterTemplate, drop2Page{Title: "Evidence", IncidentID: incidentID, Error: err.Error()})
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
	items, err := runtime.EvidenceRegister(r.Context(), token, incidentID, compartment)
	page := drop2Page{Title: "Evidence", IncidentID: incidentID, Summary: "Authorised evidence register and capture surface for the selected compartment.", Data: EvidenceRegisterPage{IncidentID: incidentID, Compartment: compartment, Items: items}}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, evidenceRegisterTemplate, page)
}

func (s *Server) drop2EvidenceCapture(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "evidence capture refused", http.StatusForbidden)
		return
	}
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	runtime, err := requireDrop2Runtime[evidenceRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	file, header, err := r.FormFile("evidence")
	if err != nil {
		http.Error(w, "evidence file is required", http.StatusBadRequest)
		return
	}
	defer file.Close()
	collectedAt, err := parseOptionalRFC3339(r.FormValue("collected_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	occurredAt, err := parseOptionalRFC3339(r.FormValue("occurred_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	incidentID := r.PathValue("incidentID")
	compartment := strings.TrimSpace(r.FormValue("compartment"))
	if compartment == "" {
		compartment = "GENERAL"
	}
	mediaType := header.Header.Get("Content-Type")
	if err := runtime.CaptureEvidence(r.Context(), sessionToken(r), incidentID, compartment, header.Filename, mediaType, strings.TrimSpace(r.FormValue("description")), strings.TrimSpace(r.FormValue("collector")), strings.TrimSpace(r.FormValue("source")), collectedAt, occurredAt, strings.TrimSpace(r.FormValue("source_location")), strings.TrimSpace(r.FormValue("current_handler")), strings.TrimSpace(r.FormValue("related_ref")), file); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/incident/"+url.PathEscape(incidentID)+"/evidence?compartment="+url.QueryEscape(compartment), http.StatusSeeOther)
}

func (s *Server) drop2EvidenceDetail(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[evidenceRuntime](s)
	if err != nil {
		s.renderSpecialist(w, evidenceDetailTemplate, drop2Page{Title: "Evidence detail", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	item, err := runtime.EvidenceDetail(r.Context(), token, r.PathValue("evidenceID"))
	if err != nil {
		s.renderSpecialist(w, evidenceDetailTemplate, drop2Page{Title: "Evidence detail", Error: err.Error()})
		return
	}
	history, err := runtime.EvidenceHistory(r.Context(), token, item.EvidenceID)
	page := drop2Page{Title: "Evidence detail", IncidentID: item.IncidentID, Summary: "Evidence metadata, authorised attachment access and chain-of-handling history.", Data: EvidenceDetailPage{Item: item, History: history}}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderSpecialist(w, evidenceDetailTemplate, page)
}

func (s *Server) drop2EvidenceHandling(w http.ResponseWriter, r *http.Request) {
	if !guardMutation(w, r) {
		return
	}
	runtime, err := requireDrop2Runtime[evidenceRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	occurredAt, err := parseOptionalRFC3339(r.FormValue("occurred_at"))
	if err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	evidenceID := r.PathValue("evidenceID")
	if err := runtime.RecordEvidenceHandling(r.Context(), sessionToken(r), evidenceID, strings.TrimSpace(r.FormValue("action")), strings.TrimSpace(r.FormValue("handler")), strings.TrimSpace(r.FormValue("location")), strings.TrimSpace(r.FormValue("note")), occurredAt); err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	http.Redirect(w, r, "/evidence/"+url.PathEscape(evidenceID), http.StatusSeeOther)
}

func (s *Server) drop2EvidenceAttachment(w http.ResponseWriter, r *http.Request) {
	runtime, err := requireDrop2Runtime[evidenceRuntime](s)
	if err != nil {
		http.Error(w, err.Error(), http.StatusNotImplemented)
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	evidenceID := r.PathValue("evidenceID")
	view, err := runtime.EvidenceDetail(r.Context(), token, evidenceID)
	if err != nil {
		http.Error(w, err.Error(), http.StatusForbidden)
		return
	}
	for key, values := range EvidenceAttachmentHeaders(view) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	if err := runtime.DownloadEvidence(r.Context(), token, evidenceID, w); err != nil {
		return
	}
}
