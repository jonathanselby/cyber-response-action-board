package web

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

var (
	boardScreen = screen{nav: "board", title: "Board", summary: "The common operating picture and live handover.", tmpl: "board", incident: true}
	closeScreen = screen{nav: "close", title: "Close-out", summary: "Recovery, stand-down, review, retention and closure.", tmpl: "close", incident: true}
)

func (s *Server) registerIncidentRoutes() {
	board := boardScreen
	board.load = s.loadBoard
	closeout := closeScreen
	closeout.load = s.loadCloseOut

	toBoard := func(r *http.Request) string { return incidentURL(r, "") }
	toClose := func(r *http.Request) string { return incidentURL(r, "/close") }
	id := func(r *http.Request) string { return r.PathValue("incidentID") }

	s.mux.HandleFunc("GET /incident/{incidentID}", s.show(board))
	s.mux.HandleFunc("POST /incident/{incidentID}/fact", s.post(board, toBoard, "fact-recorded", func(r *http.Request, token string) error {
		return s.runtime.RecordFact(r.Context(), token, id(r), formValue(r, "statement"), formValue(r, "source"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/uncertainty", s.post(board, toBoard, "uncertainty-recorded", func(r *http.Request, token string) error {
		return s.runtime.RecordUncertainty(r.Context(), token, id(r), formValue(r, "kind"), formValue(r, "statement"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/decision", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeDecision(r.Context(), token, id(r), formValue(r, "decision"), formValue(r, "rationale"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/role", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeRoleAssignment(r.Context(), token, id(r), formValue(r, "role_name"), formValue(r, "human_id"), r.FormValue("deputy") == "true")
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/action", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeAction(r.Context(), token, id(r), formValue(r, "title"), formValue(r, "owner"), formValue(r, "priority"), formValue(r, "source_ref"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/action/status", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		status := formValue(r, "status")
		if status == domain.ActionStatusSkipped && formValue(r, "skip_reason") == "" {
			return fieldError{"A reason is required to skip an action"}
		}
		return s.runtime.ProposeActionStatus(r.Context(), token, id(r), formValue(r, "action_id"), status, formValue(r, "skip_reason"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/scope", s.post(board, toBoard, "recorded", func(r *http.Request, token string) error {
		input := ScopeAssessmentInput{AffectedAssets: r.FormValue("affected_assets"), AffectedServices: r.FormValue("affected_services"), PeopleDataPopulations: r.FormValue("people_data_populations"), Locations: r.FormValue("locations"), Suppliers: r.FormValue("suppliers"), Impact: formValue(r, "impact"), Confidence: formValue(r, "confidence"), ReasonEvidence: formValue(r, "reason_evidence")}
		return s.runtime.RecordScopeAssessment(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/severity", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeSeverityChange(r.Context(), token, id(r), formValue(r, "severity"), formValue(r, "reason_evidence"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/handover", s.post(board, toBoard, "handover-recorded", func(r *http.Request, token string) error {
		return s.runtime.GenerateHandover(r.Context(), token, id(r), lines(r.FormValue("next_objectives")), lines(r.FormValue("communication_commitments")))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/resource", s.post(board, toBoard, "recorded", func(r *http.Request, token string) error {
		input := ResourceRequestInput{Category: formValue(r, "category"), Request: formValue(r, "request"), OwnerHumanID: formValue(r, "owner"), Priority: formValue(r, "priority"), Status: formValue(r, "status"), AuthoritySource: formValue(r, "authority_source"), Notes: formValue(r, "notes")}
		return s.runtime.RecordResourceRequest(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/obligation/trigger", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		at, err := parseLocalTime(r.FormValue("trigger_at"), "Trigger time", true)
		if err != nil {
			return err
		}
		value := domain.ObligationTrigger{DefinitionID: formValue(r, "definition_id"), SourceRecordID: formValue(r, "source_record_id"), SourceField: formValue(r, "source_field"), TriggerAt: *at, ApplicabilityReason: formValue(r, "applicability_reason")}
		return s.runtime.ProposeObligationTrigger(r.Context(), token, id(r), compartmentParam(r), value)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/obligation/change", s.post(board, toBoard, "proposed", func(r *http.Request, token string) error {
		deadline, err := parseLocalTime(r.FormValue("new_deadline"), "New deadline", false)
		if err != nil {
			return err
		}
		value := domain.ObligationChange{ClockID: formValue(r, "clock_id"), Kind: formValue(r, "kind"), AuthoritySource: formValue(r, "authority"), Reason: formValue(r, "reason"), NewDeadline: deadline, ReplacementDefinitionID: formValue(r, "replacement_definition_id")}
		return s.runtime.ProposeObligationChange(r.Context(), token, id(r), compartmentParam(r), value)
	}))
	s.mux.HandleFunc("GET /incident/{incidentID}/brief.pdf", s.pdf("incident-brief", s.runtime.IncidentBriefPDF))

	s.mux.HandleFunc("GET /incident/{incidentID}/close", s.show(closeout))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/recovery-update", s.post(closeout, toClose, "recorded", func(r *http.Request, token string) error {
		input := RecoveryUpdateInput{Summary: formValue(r, "summary"), OwnerHumanID: formValue(r, "owner"), RestorationPriority: formValue(r, "restoration_priority"), Verified: r.FormValue("verified") == "true", ResidualRisk: formValue(r, "residual_risk"), HandoffRef: formValue(r, "handoff_ref")}
		return s.runtime.RecordRecoveryUpdate(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/stand-down", s.post(closeout, toClose, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposeStandDown(r.Context(), token, id(r), formValue(r, "reason"))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/pir", s.post(closeout, toClose, "proposed", func(r *http.Request, token string) error {
		return s.runtime.ProposePIR(r.Context(), token, id(r), formValue(r, "executive_summary"), lines(r.FormValue("lessons")), lines(r.FormValue("long_term_work")))
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/retention", s.post(closeout, toClose, "proposed", func(r *http.Request, token string) error {
		after, err := parseLocalTime(r.FormValue("disposition_eligible_after"), "Disposition eligible after", false)
		if err != nil {
			return err
		}
		input := RetentionInput{RetentionClass: formValue(r, "retention_class"), Hold: r.FormValue("hold") == "true", HoldReason: formValue(r, "hold_reason"), DispositionEligibleAfter: after, HandoffRef: formValue(r, "handoff_ref")}
		return s.runtime.ProposeRetention(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/closure", s.post(closeout, toClose, "proposed", func(r *http.Request, token string) error {
		input := ClosureInput{Reason: formValue(r, "reason"), PrivilegedCompartment: formValue(r, "privileged_compartment"), PrivilegedSummary: formValue(r, "privileged_summary"), ShareableSummary: formValue(r, "shareable_summary")}
		return s.runtime.ProposeClosure(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("POST /incident/{incidentID}/close/transcription", s.post(closeout, toClose, "recorded", func(r *http.Request, token string) error {
		at, err := parseLocalTime(r.FormValue("paper_occurred_at"), "Original time", true)
		if err != nil {
			return err
		}
		input := PaperInput{Statement: formValue(r, "statement"), PaperOccurredAt: *at, SourceReference: formValue(r, "source_reference"), TimeConfidence: formValue(r, "time_confidence")}
		return s.runtime.TranscribePaper(r.Context(), token, id(r), input)
	}))
	s.mux.HandleFunc("GET /incident/{incidentID}/pack.pdf", s.pdf("emergency-response-pack", s.runtime.EmergencyPackPDF))

	// Removed incident screens redirect to their new home for one release.
	for _, old := range []string{"operations", "obligations", "control"} {
		s.mux.HandleFunc("GET /incident/{incidentID}/"+old, func(w http.ResponseWriter, r *http.Request) {
			http.Redirect(w, r, incidentURL(r, ""), http.StatusMovedPermanently)
		})
	}
}

func (s *Server) loadBoard(r *http.Request, token string, frame FrameView) (any, []Action, error) {
	incidentID := r.PathValue("incidentID")
	view, err := s.runtime.Board(r.Context(), token, incidentID, compartmentParam(r))
	if err != nil {
		return nil, nil, err
	}
	data := struct {
		Board    BoardView
		Guidance *GuidanceView
	}{Board: view}
	if q := strings.TrimSpace(r.URL.Query().Get("q")); q != "" {
		guidance, err := s.runtime.Retrieve(r.Context(), token, incidentID, q)
		if err != nil {
			guidance = GuidanceView{Question: q, Refused: true, Reason: refusalText(err)}
		}
		guidance.Question = q
		data.Guidance = &guidance
	}
	actions := []Action{
		{Label: "Record fact", URL: "?open=record-fact#record-fact", Primary: true},
		{Label: "Incident Brief (PDF)", URL: incidentURL(r, "/brief.pdf")},
		{Label: "Record handover", URL: "?open=handover#handover"},
	}
	_ = frame
	return data, actions, nil
}

func (s *Server) loadCloseOut(r *http.Request, token string, _ FrameView) (any, []Action, error) {
	view, err := s.runtime.CloseOut(r.Context(), token, r.PathValue("incidentID"))
	if err != nil {
		return nil, nil, err
	}
	var primary Action
	switch {
	case view.State.ClosedAt != nil:
		primary = Action{}
	case view.State.ClosureReady:
		primary = Action{Label: "Propose closure", URL: "?open=closure#closure", Primary: true}
	case view.State.StandDown != nil && view.State.PIR == nil:
		primary = Action{Label: "Start PIR", URL: "?open=pir#pir", Primary: true}
	case view.State.StandDown != nil:
		primary = Action{Label: "Propose retention", URL: "?open=retention#retention", Primary: true}
	default:
		primary = Action{Label: "Record recovery update", URL: "?open=recovery#recovery", Primary: true}
	}
	actions := []Action{{Label: "Emergency Pack (PDF)", URL: incidentURL(r, "/pack.pdf")}}
	if primary.Label != "" {
		actions = append([]Action{primary}, actions...)
	}
	return view, actions, nil
}

// pdf streams an operational PDF generated by the application print service.
// The separate print permission is enforced in the application tier.
func (s *Server) pdf(name string, generate func(ctx context.Context, token, incidentID, pageSize, printMode string) ([]byte, error)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		token := sessionToken(r)
		if token == "" {
			http.Redirect(w, r, "/", http.StatusSeeOther)
			return
		}
		pageSize := strings.ToUpper(strings.TrimSpace(r.URL.Query().Get("size")))
		mode := strings.TrimSpace(r.URL.Query().Get("mode"))
		if mode == "" {
			mode = domain.PrintColour
		}
		incidentID := r.PathValue("incidentID")
		body, err := generate(r.Context(), token, incidentID, pageSize, mode)
		if err != nil {
			s.render(w, r, token, screen{nav: "board", title: "Paper", tmpl: "refused", incident: true, load: func(*http.Request, string, FrameView) (any, []Action, error) { return nil, nil, err }}, refusalStatus(err), pageData{})
			return
		}
		filename := safeEvidenceFilename(incidentID + "-" + name + "-" + time.Now().UTC().Format("20060102T1504Z") + ".pdf")
		writeAttachment(w, filename, "application/pdf", body)
	}
}
