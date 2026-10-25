package web

import (
	"net/http"
	"time"
)

var accessScreen = screen{nav: "access", title: "Access", summary: "Who may work on this incident and its restricted compartments.", tmpl: "access", incident: true}

func (s *Server) registerAccessRoutes() {
	sc := accessScreen
	sc.load = func(r *http.Request, token string, _ FrameView) (any, []Action, error) {
		view, err := s.runtime.IncidentAccess(r.Context(), token, r.PathValue("incidentID"), time.Now().UTC())
		return view, []Action{{Label: "Request access", URL: "?open=member#member", Primary: true}, {Label: "Read into compartment", URL: "?open=compartment#compartment"}}, err
	}
	back := func(r *http.Request) string { return incidentURL(r, "/access") }
	id := func(r *http.Request) string { return r.PathValue("incidentID") }
	window := func(r *http.Request) (*time.Time, *time.Time, error) {
		effective, err := parseLocalTime(r.FormValue("effective_at"), "Effective from", false)
		if err != nil {
			return nil, nil, err
		}
		expires, err := parseLocalTime(r.FormValue("expires_at"), "Expires at", false)
		return effective, expires, err
	}

	s.mux.HandleFunc("GET /incident/{incidentID}/access", s.show(sc))
	s.mux.HandleFunc("POST /incident/{incidentID}/access/member", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		effective, expires, err := window(r)
		if err != nil {
			return err
		}
		return s.runtime.ProposeIncidentMembership(r.Context(), token, id(r), formValue(r, "human_id"), r.FormValue("active") == "true", effective, expires, formValue(r, "reason"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/access/sponsor", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeSponsor(r.Context(), token, id(r), formValue(r, "human_id"), r.FormValue("active") == "true")
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/access/controller", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeController(r.Context(), token, id(r), formValue(r, "human_id"), formValue(r, "compartment_name"), r.FormValue("active") == "true")
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/access/compartment-member", s.post(sc, back, "proposed", func(r *http.Request, token string) error {
		effective, expires, err := window(r)
		if err != nil {
			return err
		}
		return s.runtime.ProposeCompartmentMembership(r.Context(), token, id(r), formValue(r, "human_id"), formValue(r, "compartment_name"), r.FormValue("active") == "true", effective, expires, formValue(r, "reason"))
	}))
}
