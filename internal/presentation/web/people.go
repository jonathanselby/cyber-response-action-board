package web

import (
	"net/http"
	"net/url"
)

var (
	enrolScreen     = screen{title: "Set up your account", summary: "Use the enrolment code a Custodian gave you.", tmpl: "enrol"}
	resetCodeScreen = screen{nav: "access-admin", title: "Enrolment code", summary: "Shown once. Pass it to the person directly.", tmpl: "reset-code"}
)

// registerPeopleRoutes adds people administration and first-time enrolment.
// Every decision is made by the runtime; these handlers parse and render.
func (s *Server) registerPeopleRoutes(admin, approvals screen) {
	back := func(*http.Request) string { return "/access" }
	s.mux.HandleFunc("POST /access/person", s.post(admin, back, "proposed", func(r *http.Request, token string) error {
		in := PersonInput{HumanID: formValue(r, "human_id"), DisplayName: formValue(r, "display_name"), Organisation: formValue(r, "organisation"), Role: formValue(r, "role"), External: r.FormValue("external") == "true"}
		expires, err := parseLocalTime(r.FormValue("account_expires_at"), "Account expiry", false)
		if err != nil {
			return err
		}
		in.AccountExpiresAt = expires
		return s.runtime.ProposePerson(r.Context(), token, in)
	}))
	s.mux.HandleFunc("POST /access/person/status", s.post(admin, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposePersonStatus(r.Context(), token, formValue(r, "human_id"), formValue(r, "status"), formValue(r, "reason"))
	}))
	s.mux.HandleFunc("POST /access/person/reset", s.post(admin, back, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeCredentialReset(r.Context(), token, formValue(r, "human_id"), formValue(r, "reason"))
	}))
	s.mux.HandleFunc("POST /access/reset/confirm", s.confirmReset(approvals))
	s.mux.HandleFunc("GET /enrol", s.enrolForm)
	s.mux.HandleFunc("POST /enrol/begin", s.enrolBegin)
	s.mux.HandleFunc("POST /enrol/complete", s.enrolComplete)
}

// confirmReset renders the enrolment code in the response body only: never in
// a URL, never stored by the browser (authenticated pages are no-store).
func (s *Server) confirmReset(approvals screen) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			http.Error(w, "authentication required", http.StatusUnauthorized)
			return
		}
		if !sameOrigin(r) {
			http.Error(w, "request origin refused", http.StatusForbidden)
			return
		}
		if err := parseSmallForm(w, r); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		view, err := s.runtime.ConfirmCredentialReset(r.Context(), token, formValue(r, "record_id"), formValue(r, "version"))
		if err != nil {
			s.render(w, r, token, approvals, refusalStatus(err), pageData{Error: refusalText(err)})
			return
		}
		s.render(w, r, token, resetCodeScreen, http.StatusOK, pageData{Data: view})
	}
}

func (s *Server) enrolForm(w http.ResponseWriter, r *http.Request) {
	s.page(w, http.StatusOK, enrolScreen, pageData{Form: url.Values{"human_id": {r.URL.Query().Get("human_id")}}}, nil)
}

func (s *Server) enrolBegin(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "request origin refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	humanID, code := formValue(r, "human_id"), formValue(r, "code")
	view, err := s.runtime.BeginEnrolment(r.Context(), humanID, code)
	if err != nil {
		s.page(w, http.StatusUnauthorized, enrolScreen, pageData{Error: "That enrolment code was not accepted. Check it with the Custodian who gave it to you.", Form: url.Values{"human_id": {humanID}}}, nil)
		return
	}
	s.page(w, http.StatusOK, enrolScreen, pageData{Data: view}, nil)
}

func (s *Server) enrolComplete(w http.ResponseWriter, r *http.Request) {
	if !sameOrigin(r) {
		http.Error(w, "request origin refused", http.StatusForbidden)
		return
	}
	if err := parseSmallForm(w, r); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	humanID, code := formValue(r, "human_id"), formValue(r, "code")
	token, err := s.runtime.CompleteEnrolment(r.Context(), humanID, code, r.FormValue("password"), formValue(r, "totp"))
	if err != nil {
		// Show the second step again with the refusal; the secret is re-issued
		// so a stale authenticator entry cannot linger.
		view, berr := s.runtime.BeginEnrolment(r.Context(), humanID, code)
		if berr != nil {
			s.page(w, http.StatusUnauthorized, enrolScreen, pageData{Error: "That enrolment code was not accepted.", Form: url.Values{"human_id": {humanID}}}, nil)
			return
		}
		s.page(w, http.StatusUnprocessableEntity, enrolScreen, pageData{Data: view, Error: refusalText(err)}, nil)
		return
	}
	s.setSession(w, r, token)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
