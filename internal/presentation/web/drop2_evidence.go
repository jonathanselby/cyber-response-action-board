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

var evidenceRegisterTemplate = template.Must(template.New("evidence-register").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Evidence · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Evidence</h1></div><div class="version">{{.Version}}</div></header><main><nav class="crumb"><a href="/">Incidents</a><span>/</span><a href="/incident/{{.IncidentID}}">{{.IncidentID}}</a><span>/</span><strong>Evidence</strong></nav>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<section class="panel"><form method="get" action="/incident/{{.IncidentID}}/evidence" class="form-grid"><label>Compartment<input name="compartment" value="{{.Compartment}}"></label><button>Open register</button></form></section><section class="dashboard-grid"><article><h3>Register</h3>{{range .Items}}<p><a href="/evidence/{{.EvidenceID}}"><strong>{{.Evidence.OriginalFilename}}</strong></a><br><small>{{.EvidenceID}} · {{.Compartment}} · {{.Evidence.SizeBytes}} bytes · {{.Evidence.DetectedMediaType}}</small></p>{{else}}<p class="muted">No evidence in this authorised scope.</p>{{end}}</article><article><h3>Capture evidence</h3><form method="post" enctype="multipart/form-data" action="/incident/{{.IncidentID}}/evidence" class="form-grid"><input type="hidden" name="compartment" value="{{.Compartment}}"><label>File<input type="file" name="evidence" accept="image/*,audio/*,video/*,*/*" capture required></label><label>Description<textarea name="description"></textarea></label><label>Collector<input name="collector" required></label><label>Source<input name="source" required></label><label>Current handler<input name="current_handler" required></label><label>Source location<input name="source_location"></label><label>Related reference<input name="related_ref"></label><label>Collected at (RFC3339)<input name="collected_at"></label><label>Occurred at (RFC3339)<input name="occurred_at"></label><button>Preserve evidence</button></form></article></section>{{end}}</main></body></html>`))

var evidenceDetailTemplate = template.Must(template.New("evidence-detail").Parse(`<!doctype html><html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width,initial-scale=1"><title>Evidence · CRAB</title><link rel="stylesheet" href="/static/app.css"></head><body><header class="shell"><div><p class="eyebrow">Cyber Response Action Board</p><h1>Evidence detail</h1></div><div class="version">{{.Version}}</div></header><main>{{if .Error}}<section class="notice notice--error"><p>{{.Error}}</p></section>{{end}}{{with .Data}}<nav class="crumb"><a href="/incident/{{.Item.IncidentID}}">{{.Item.IncidentID}}</a><span>/</span><a href="/incident/{{.Item.IncidentID}}/evidence?compartment={{.Item.Compartment}}">Evidence</a><span>/</span><strong>{{.Item.EvidenceID}}</strong></nav><section class="dashboard-grid"><article><h3>{{.Item.Evidence.OriginalFilename}}</h3><p>{{.Item.Evidence.Description}}</p><p><small>SHA-256 {{.Item.Evidence.SHA256}}<br>{{.Item.Evidence.SizeBytes}} bytes · declared {{.Item.Evidence.DeclaredMediaType}} · detected {{.Item.Evidence.DetectedMediaType}}<br>collector {{.Item.Evidence.Collector}} · source {{.Item.Evidence.Source}} · handler {{.Item.Evidence.CurrentHandler}}</small></p><p><a href="/evidence/{{.Item.EvidenceID}}/attachment">Authorised view/download</a></p></article><article><h3>Handling history</h3>{{range .History}}<p><strong>{{.Handling.Action}}</strong><br><small>{{.Handling.Handler}} · {{.RecordedAt}} {{if .Handling.Location}}· {{.Handling.Location}}{{end}}</small><br>{{.Handling.Note}}</p>{{else}}<p class="muted">No handling records.</p>{{end}}</article><article><h3>Record handling</h3><form method="post" action="/evidence/{{.Item.EvidenceID}}/handling" class="form-grid"><label>Action<input name="action" required></label><label>Handler<input name="handler" required></label><label>Location<input name="location"></label><label>Occurred at (RFC3339)<input name="occurred_at"></label><label>Note<textarea name="note"></textarea></label><button>Record</button></form></article></section>{{end}}</main></body></html>`))

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
		s.renderDrop2(w, evidenceRegisterTemplate, drop2Page{Title: "Evidence", IncidentID: incidentID, Error: err.Error()})
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
	page := drop2Page{Title: "Evidence", IncidentID: incidentID, Data: EvidenceRegisterPage{IncidentID: incidentID, Compartment: compartment, Items: items}}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, evidenceRegisterTemplate, page)
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
		s.renderDrop2(w, evidenceDetailTemplate, drop2Page{Title: "Evidence detail", Error: err.Error()})
		return
	}
	token, err := drop2Token(r)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusSeeOther)
		return
	}
	item, err := runtime.EvidenceDetail(r.Context(), token, r.PathValue("evidenceID"))
	if err != nil {
		s.renderDrop2(w, evidenceDetailTemplate, drop2Page{Title: "Evidence detail", Error: err.Error()})
		return
	}
	history, err := runtime.EvidenceHistory(r.Context(), token, item.EvidenceID)
	page := drop2Page{Title: "Evidence detail", IncidentID: item.IncidentID, Data: EvidenceDetailPage{Item: item, History: history}}
	if err != nil {
		page.Data = nil
		page.Error = err.Error()
	}
	s.renderDrop2(w, evidenceDetailTemplate, page)
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
