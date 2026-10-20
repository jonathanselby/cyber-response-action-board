package web

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
)

const maxAPIRequestBytes = 64 << 10

func (s *Server) registerAPIRoutes() {
	s.mux.HandleFunc("GET /api/v1/incidents", s.apiIncidents)
	s.mux.HandleFunc("GET /api/v1/incidents/{incidentID}", s.apiIncident)
	s.mux.HandleFunc("POST /api/v1/incidents", s.apiDeclareIncident)
	s.mux.HandleFunc("POST /api/v1/incidents/{incidentID}/facts", s.apiRecordFact)
	s.mux.HandleFunc("POST /api/v1/incidents/{incidentID}/actions", s.apiProposeAction)
	s.mux.HandleFunc("POST /api/v1/incidents/{incidentID}/decisions", s.apiDecision)
}

func (s *Server) apiIncidents(w http.ResponseWriter, r *http.Request) {
	token, ok := apiToken(w, r)
	if !ok {
		return
	}
	home, authenticated, err := s.runtime.Home(r.Context(), token)
	if err != nil || !authenticated {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusOK, home)
}

func (s *Server) apiIncident(w http.ResponseWriter, r *http.Request) {
	token, ok := apiToken(w, r)
	if !ok {
		return
	}
	view, found, err := s.runtime.Incident(r.Context(), token, r.PathValue("incidentID"))
	if err != nil || !found {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusOK, view)
}

func (s *Server) apiDeclareIncident(w http.ResponseWriter, r *http.Request) {
	token, ok := apiMutationToken(w, r)
	if !ok {
		return
	}
	var input struct {
		IncidentID string `json:"incident_id"`
		Mode       string `json:"mode"`
		Title      string `json:"title"`
		Summary    string `json:"summary"`
		Severity   string `json:"severity"`
	}
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	if err := s.runtime.DeclareIncident(r.Context(), token, strings.TrimSpace(input.IncidentID), strings.TrimSpace(input.Mode), strings.TrimSpace(input.Title), strings.TrimSpace(input.Summary), strings.TrimSpace(input.Severity)); err != nil {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusAccepted, map[string]string{"status": "proposed"})
}

func (s *Server) apiRecordFact(w http.ResponseWriter, r *http.Request) {
	token, ok := apiMutationToken(w, r)
	if !ok {
		return
	}
	var input struct {
		Statement string `json:"statement"`
		Source    string `json:"source"`
	}
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	if err := s.runtime.RecordFact(r.Context(), token, r.PathValue("incidentID"), strings.TrimSpace(input.Statement), strings.TrimSpace(input.Source)); err != nil {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusCreated, map[string]string{"status": "recorded"})
}

func (s *Server) apiProposeAction(w http.ResponseWriter, r *http.Request) {
	token, ok := apiMutationToken(w, r)
	if !ok {
		return
	}
	var input struct {
		Title     string `json:"title"`
		Owner     string `json:"owner"`
		Priority  string `json:"priority"`
		SourceRef string `json:"source_ref"`
	}
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	if err := s.runtime.ProposeAction(r.Context(), token, r.PathValue("incidentID"), strings.TrimSpace(input.Title), strings.TrimSpace(input.Owner), strings.TrimSpace(input.Priority), strings.TrimSpace(input.SourceRef)); err != nil {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusAccepted, map[string]string{"status": "proposed"})
}

func (s *Server) apiDecision(w http.ResponseWriter, r *http.Request) {
	token, ok := apiMutationToken(w, r)
	if !ok {
		return
	}
	var input struct {
		RecordType string `json:"record_type"`
		RecordID   string `json:"record_id"`
		Version    string `json:"version"`
		Outcome    string `json:"outcome"`
	}
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	if err := s.runtime.Decide(r.Context(), token, r.PathValue("incidentID"), strings.TrimSpace(input.RecordType), strings.TrimSpace(input.RecordID), strings.TrimSpace(input.Version), strings.TrimSpace(input.Outcome)); err != nil {
		apiDenied(w)
		return
	}
	writeAPIJSON(w, http.StatusOK, map[string]string{"status": strings.TrimSpace(input.Outcome)})
}

func apiToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	auth := strings.TrimSpace(r.Header.Get("Authorization"))
	const prefix = "Bearer "
	if !strings.HasPrefix(auth, prefix) {
		apiDenied(w)
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(auth, prefix))
	if token == "" {
		apiDenied(w)
		return "", false
	}
	return token, true
}

func apiMutationToken(w http.ResponseWriter, r *http.Request) (string, bool) {
	if r.TLS == nil && !loopbackRequest(r) {
		http.Error(w, "TLS required", http.StatusForbidden)
		return "", false
	}
	return apiToken(w, r)
}

func decodeAPIJSON(w http.ResponseWriter, r *http.Request, target any) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxAPIRequestBytes)
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(target); err != nil {
		http.Error(w, "invalid JSON request", http.StatusBadRequest)
		return false
	}
	var extra any
	if err := dec.Decode(&extra); !errors.Is(err, io.EOF) {
		http.Error(w, "request must contain one JSON object", http.StatusBadRequest)
		return false
	}
	return true
}

func apiDenied(w http.ResponseWriter) {
	http.Error(w, "not found", http.StatusNotFound)
}

func writeAPIJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
