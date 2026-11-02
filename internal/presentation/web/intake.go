package web

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

var intakeKeyScreen = screen{nav: "access-admin", title: "Intake key secret", summary: "Shown once. Put it straight into the sending system's secret store.", tmpl: "intake-key"}

// Intake outcomes, decided by the runtime and mapped to HTTP here.
const (
	IntakeCreated     = "created"
	IntakeExisting    = "existing"
	IntakeRefused     = "refused"
	IntakeRateLimited = "rate_limited"
	IntakeInvalid     = "invalid"
)

// IntakeReportInput is the body of POST /api/v1/first-reports.
type IntakeReportInput struct {
	Description     string     `json:"description"`
	Reporter        string     `json:"reporter"`
	ContactRoute    string     `json:"contact_route"`
	Affected        string     `json:"affected"`
	ImmediateImpact string     `json:"immediate_impact"`
	ReceivedAt      *time.Time `json:"received_at"`
	TimeConfidence  string     `json:"time_confidence"`
	ExternalRef     string     `json:"external_ref"`
}

// IntakeResult is the runtime's answer to a machine submission.
type IntakeResult struct {
	Outcome  string
	ReportID string
	Message  string
}

// IntakeKeyInput is a Custodian's intake key proposal form.
type IntakeKeyInput struct {
	KeyName, Action, AccountableHuman, Reason string
	ExpiresAt                                 *time.Time
	OverlapMinutes                            int
}

// IntakeKeyRow is one intake key in access administration.
type IntakeKeyRow struct {
	KeyName, AccountableHuman, Status, ExpiresAt, IssuedAt, RotatedAt, PreviousUntil string
	CanRotate, CanRevoke                                                             bool
}

// IntakeKeySecretView is shown once to the confirming Custodian.
type IntakeKeySecretView struct {
	KeyName, AccountableHuman, Action, Secret, ExpiresAt, PreviousUntil string
}

func (s *Server) registerIntakeRoutes(admin, approvals screen) {
	s.mux.HandleFunc("POST /api/v1/first-reports", s.apiFirstReport)
	back := func(*http.Request) string { return "/access#intake-keys" }
	s.mux.HandleFunc("POST /access/intake-key", s.post(admin, back, "proposed", func(r *http.Request, token string) error {
		in := IntakeKeyInput{KeyName: formValue(r, "key_name"), Action: formValue(r, "action"), AccountableHuman: formValue(r, "accountable_human"), Reason: formValue(r, "reason")}
		expires, err := parseLocalTime(r.FormValue("expires_at"), "Key expiry", false)
		if err != nil {
			return err
		}
		in.ExpiresAt = expires
		if hours := strings.TrimSpace(r.FormValue("overlap_hours")); hours != "" {
			n, err := strconv.Atoi(hours)
			if err != nil {
				return fieldError{"Overlap must be a whole number of hours"}
			}
			in.OverlapMinutes = n * 60
		}
		return s.runtime.ProposeIntakeKey(r.Context(), token, in)
	}))
	s.mux.HandleFunc("POST /access/intake-key/confirm", s.confirmIntakeKey(approvals))
}

// apiFirstReport is the machine intake endpoint (addendum A4). Every refusal
// of the key is the same response as any other API refusal.
func (s *Server) apiFirstReport(w http.ResponseWriter, r *http.Request) {
	token, ok := apiMutationToken(w, r)
	if !ok {
		return
	}
	var input IntakeReportInput
	if !decodeAPIJSON(w, r, &input) {
		return
	}
	result, err := s.runtime.SubmitIntakeReport(r.Context(), token, clientSource(r), input)
	if err != nil {
		http.Error(w, "first report not recorded", http.StatusInternalServerError)
		return
	}
	switch result.Outcome {
	case IntakeCreated:
		writeAPIJSON(w, http.StatusCreated, map[string]string{"report_id": result.ReportID, "status": "recorded"})
	case IntakeExisting:
		writeAPIJSON(w, http.StatusOK, map[string]string{"report_id": result.ReportID, "status": "already recorded"})
	case IntakeRateLimited:
		w.Header().Set("Retry-After", "60")
		http.Error(w, "too many requests", http.StatusTooManyRequests)
	case IntakeInvalid:
		http.Error(w, result.Message, http.StatusBadRequest)
	default:
		apiDenied(w)
	}
}

// confirmIntakeKey renders a new secret in the response body only: never in a
// URL, never stored by the browser (authenticated pages are no-store).
func (s *Server) confirmIntakeKey(approvals screen) http.HandlerFunc {
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
		view, err := s.runtime.ConfirmIntakeKey(r.Context(), token, formValue(r, "record_id"), formValue(r, "version"))
		if err != nil {
			s.render(w, r, token, approvals, refusalStatus(err), pageData{Error: refusalText(err)})
			return
		}
		s.render(w, r, token, intakeKeyScreen, http.StatusOK, pageData{Data: view})
	}
}
