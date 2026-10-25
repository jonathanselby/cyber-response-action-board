package web

import (
	"net/http"
	"net/url"
	"strconv"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

var communicationsScreen = screen{nav: "communications", title: "Communications", summary: "Structured incident communications, approvals and delivery outcomes.", tmpl: "communications", incident: true}

func (s *Server) registerCommunicationRoutes() {
	sc := communicationsScreen
	sc.load = func(r *http.Request, token string, frame FrameView) (any, []Action, error) {
		incidentID := r.PathValue("incidentID")
		view, err := s.runtime.Communications(r.Context(), token, incidentID, compartmentParam(r))
		if err != nil {
			return nil, nil, err
		}
		if frame.Incident != nil {
			view.Compartments = frame.Incident.Compartments
		}
		return view, []Action{{Label: "Draft communication", URL: "?open=draft#draft", Primary: true}, {Label: "Record enquiry", URL: "?open=enquiry#enquiry"}}, nil
	}
	back := func(r *http.Request) string {
		return incidentURL(r, "/communications?compartment="+url.QueryEscape(compartmentParam(r)))
	}
	id := func(r *http.Request) string { return r.PathValue("incidentID") }

	s.mux.HandleFunc("GET /incident/{incidentID}/communications", s.show(sc))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/draft", s.post(sc, back, "drafted", func(r *http.Request, token string) error {
		value := domain.CommunicationDraft{Kind: formValue(r, "kind"), Subject: formValue(r, "subject"), Body: formValue(r, "body"), Audience: lines(r.FormValue("audience")), Channels: lines(r.FormValue("channels")), SourceRefs: lines(r.FormValue("source_refs")), RelatedRecordIDs: lines(r.FormValue("related_record_ids"))}
		return s.runtime.RecordCommunicationDraft(r.Context(), token, id(r), compartmentParam(r), value)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/issue", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeCommunicationIssue(r.Context(), token, id(r), formValue(r, "draft_id"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/cadence", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		interval, err := strconv.Atoi(formValue(r, "interval_minutes"))
		if err != nil || interval <= 0 {
			return fieldError{"Interval must be a positive number of minutes"}
		}
		first, err := parseLocalTime(r.FormValue("first_due_at"), "First report due", true)
		if err != nil {
			return err
		}
		value := domain.CommunicationCadence{Name: formValue(r, "name"), IntervalMinutes: interval, FirstDueAt: *first, OwnerHumanID: formValue(r, "owner"), Audience: lines(r.FormValue("audience")), Channels: lines(r.FormValue("channels"))}
		return s.runtime.ProposeCommunicationCadence(r.Context(), token, id(r), compartmentParam(r), value)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/enquiry", s.post(sc, back, "recorded", func(r *http.Request, token string) error {
		received, err := parseLocalTime(r.FormValue("received_at"), "Received at", true)
		if err != nil {
			return err
		}
		value := domain.IncomingEnquiry{Source: formValue(r, "source"), Contact: formValue(r, "contact"), Subject: formValue(r, "subject"), Message: formValue(r, "message"), ReceivedAt: *received, AssignedToHumanID: formValue(r, "assigned_to"), RelatedRecordIDs: lines(r.FormValue("related_record_ids"))}
		return s.runtime.RecordIncomingEnquiry(r.Context(), token, id(r), compartmentParam(r), value)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/sitrep", s.post(sc, back, "drafted", func(r *http.Request, token string) error {
		return s.runtime.GenerateSituationReport(r.Context(), token, id(r), formValue(r, "cadence_id"), "GENERAL")
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/communications/notify", s.post(sc, back, "notified", func(r *http.Request, token string) error {
		return s.runtime.NotifyCommunication(r.Context(), token, id(r), formValue(r, "communication_id"), lines(r.FormValue("channels")))
	}))
}
