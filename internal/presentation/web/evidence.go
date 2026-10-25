package web

import (
	"net/http"
	"net/url"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

var (
	evidenceScreen       = screen{nav: "evidence", title: "Evidence", summary: "Preserved original bytes, metadata and handling history.", tmpl: "evidence", incident: true}
	evidenceDetailScreen = screen{nav: "evidence", title: "Evidence item", tmpl: "evidence-detail"}
)

func (s *Server) registerEvidenceRoutes() {
	register := evidenceScreen
	register.load = s.loadEvidence
	detail := evidenceDetailScreen
	detail.load = s.loadEvidenceDetail

	s.mux.HandleFunc("GET /incident/{incidentID}/evidence", s.show(register))
	s.mux.HandleFunc("POST /incident/{incidentID}/evidence", s.post(register, func(r *http.Request) string {
		return incidentURL(r, "/evidence?compartment="+url.QueryEscape(compartmentParam(r)))
	}, "evidence-preserved", s.captureEvidence))
	s.mux.HandleFunc("GET /evidence/{evidenceID}", s.showEvidenceDetail(detail))
	s.mux.HandleFunc("POST /evidence/{evidenceID}/handling", s.post(detail, func(r *http.Request) string {
		return "/evidence/" + url.PathEscape(r.PathValue("evidenceID"))
	}, "recorded", func(r *http.Request, token string) error {
		at, err := parseLocalTime(r.FormValue("occurred_at"), "Occurred at", false)
		if err != nil {
			return err
		}
		return s.runtime.RecordEvidenceHandling(r.Context(), token, r.PathValue("evidenceID"), formValue(r, "action"), formValue(r, "handler"), formValue(r, "location"), formValue(r, "note"), at)
	}))
	s.mux.HandleFunc("GET /evidence/{evidenceID}/attachment", s.evidenceAttachment)
}

func (s *Server) loadEvidence(r *http.Request, token string, frame FrameView) (any, []Action, error) {
	incidentID := r.PathValue("incidentID")
	compartment := compartmentParam(r)
	items, err := s.runtime.EvidenceRegister(r.Context(), token, incidentID, compartment)
	if err != nil {
		return nil, nil, err
	}
	compartments := []string{"GENERAL"}
	if frame.Incident != nil && len(frame.Incident.Compartments) > 0 {
		compartments = frame.Incident.Compartments
	}
	return EvidenceRegisterPage{IncidentID: incidentID, Compartment: compartment, Compartments: compartments, Items: items}, []Action{{Label: "Add evidence", URL: "?open=add#add", Primary: true}}, nil
}

// showEvidenceDetail places the evidence page in its incident's frame once the
// application tier has authorised the item.
func (s *Server) showEvidenceDetail(sc screen) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		item, err := s.runtime.EvidenceDetail(r.Context(), token, r.PathValue("evidenceID"))
		if err != nil {
			s.render(w, r, token, screen{nav: "evidence", title: "Evidence item", tmpl: "refused", load: func(*http.Request, string, FrameView) (any, []Action, error) { return nil, nil, err }}, refusalStatus(err), pageData{})
			return
		}
		r.SetPathValue("incidentID", item.IncidentID)
		sc.incident = true
		data := pageData{Notice: messages[r.URL.Query().Get("m")]}
		s.render(w, r, token, sc, http.StatusOK, data)
	}
}

func (s *Server) loadEvidenceDetail(r *http.Request, token string, _ FrameView) (any, []Action, error) {
	evidenceID := r.PathValue("evidenceID")
	item, err := s.runtime.EvidenceDetail(r.Context(), token, evidenceID)
	if err != nil {
		return nil, nil, err
	}
	history, err := s.runtime.EvidenceHistory(r.Context(), token, evidenceID)
	if err != nil {
		return nil, nil, err
	}
	page := EvidenceDetailPage{Item: item, History: history}
	if domain.EvidencePresentationMode(item.Evidence.DeclaredMediaType, item.Evidence.DetectedMediaType) == domain.EvidencePresentationInline {
		switch {
		case strings.HasPrefix(item.Evidence.DetectedMediaType, "image/"):
			page.Inline = "image"
		case strings.HasPrefix(item.Evidence.DetectedMediaType, "audio/"):
			page.Inline = "audio"
		case strings.HasPrefix(item.Evidence.DetectedMediaType, "video/"):
			page.Inline = "video"
		}
	}
	return page, []Action{{Label: "Download original", URL: "/evidence/" + url.PathEscape(evidenceID) + "/attachment", Primary: true}}, nil
}

func (s *Server) captureEvidence(r *http.Request, token string) error {
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		return fieldError{"the upload could not be read"}
	}
	defer func() {
		if r.MultipartForm != nil {
			_ = r.MultipartForm.RemoveAll()
		}
	}()
	var picked string
	for _, field := range []string{"camera", "audio", "video", "evidence"} {
		if files := r.MultipartForm.File[field]; len(files) > 0 && files[0] != nil && strings.TrimSpace(files[0].Filename) != "" {
			picked = field
			break
		}
	}
	if picked == "" {
		return fieldError{"Choose a file, or take a photo, audio or video"}
	}
	file, header, err := r.FormFile(picked)
	if err != nil {
		return fieldError{"the evidence file could not be opened"}
	}
	defer file.Close()
	collectedAt, err := parseLocalTime(r.FormValue("collected_at"), "Collected at", false)
	if err != nil {
		return err
	}
	occurredAt, err := parseLocalTime(r.FormValue("occurred_at"), "Occurred at", false)
	if err != nil {
		return err
	}
	return s.runtime.CaptureEvidence(r.Context(), token, r.PathValue("incidentID"), compartmentParam(r), header.Filename, header.Header.Get("Content-Type"), formValue(r, "description"), formValue(r, "collector"), formValue(r, "source"), collectedAt, occurredAt, formValue(r, "source_location"), formValue(r, "current_handler"), formValue(r, "related_ref"), file)
}

func (s *Server) evidenceAttachment(w http.ResponseWriter, r *http.Request) {
	token := sessionToken(r)
	if token == "" {
		http.Error(w, "authentication required", http.StatusUnauthorized)
		return
	}
	evidenceID := r.PathValue("evidenceID")
	view, err := s.runtime.EvidenceDetail(r.Context(), token, evidenceID)
	if err != nil {
		http.Error(w, "evidence unavailable", http.StatusForbidden)
		return
	}
	for key, values := range EvidenceAttachmentHeaders(view) {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	_ = s.runtime.DownloadEvidence(r.Context(), token, evidenceID, w)
}
