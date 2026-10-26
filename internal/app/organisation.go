package app

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// OrganisationBackend persists organisation configuration through the ledger.
type OrganisationBackend interface {
	AppendOrganisation(context.Context, ledger.Record) error
	PendingOrganisation(context.Context, string) (ledger.Record, bool, error)
	PendingOrganisationList(context.Context) ([]domain.OrganisationView, error)
	CurrentOrganisation(context.Context, string) ([]domain.OrganisationView, error)
}

// OrganisationService governs organisational policy: severity scale,
// emergency configuration, checklist templates and single-person windows.
// Majors propose; a different Major confirms (UAT2-BUILD-NOTES 4).
type OrganisationService struct {
	Backend OrganisationBackend
	// MajorEligible checks that people named in a window are Majors.
	MajorEligible func(context.Context, string) (bool, error)
}

var ErrSeverityScaleMissing = errors.New("define the severity scale first: Organisation → Severity scale")

func (s OrganisationService) requireMajor(principal authz.Principal) error {
	if s.Backend == nil {
		return errors.New("organisation backend is required")
	}
	if !principal.Authenticated || principal.HumanID == "" || principal.Role != authz.RoleMajor {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s OrganisationService) append(ctx context.Context, record ledger.Record, err error) (ledger.Record, error) {
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendOrganisation(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s OrganisationService) ProposeSeverityScale(ctx context.Context, principal authz.Principal, recordID string, scale domain.SeverityScale, now time.Time) (ledger.Record, error) {
	if err := s.requireMajor(principal); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeSeverityScale(recordID, principal.HumanID, now, scale)
	return s.append(ctx, record, err)
}

func (s OrganisationService) ProposeEmergencyConfiguration(ctx context.Context, principal authz.Principal, recordID string, c domain.EmergencyConfiguration, now time.Time) (ledger.Record, error) {
	if err := s.requireMajor(principal); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeEmergencyConfiguration(recordID, principal.HumanID, now, c)
	return s.append(ctx, record, err)
}

func (s OrganisationService) ProposeChecklistTemplate(ctx context.Context, principal authz.Principal, recordID string, t domain.GovernedChecklistTemplate, now time.Time) (ledger.Record, error) {
	if err := s.requireMajor(principal); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeChecklistTemplate(recordID, principal.HumanID, now, t)
	return s.append(ctx, record, err)
}

// Decide confirms or rejects an organisation proposal. The maker can never
// confirm their own policy change, inside or outside a single-person window.
func (s OrganisationService) Decide(ctx context.Context, principal authz.Principal, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.requireMajor(principal); err != nil {
		return ledger.Record{}, err
	}
	proposal, found, err := s.Backend.PendingOrganisation(ctx, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	decided, err := domain.Decide(proposal, expectedVersion, principal.HumanID, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendOrganisation(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

// Pending lists organisation proposals for Majors.
func (s OrganisationService) Pending(ctx context.Context, principal authz.Principal) ([]domain.OrganisationView, error) {
	if err := s.requireMajor(principal); err != nil {
		return nil, err
	}
	return s.Backend.PendingOrganisationList(ctx)
}

// SeverityScale is readable by any authenticated person: it is the shared
// vocabulary every responder selects from.
func (s OrganisationService) SeverityScale(ctx context.Context) (domain.SeverityScale, bool, error) {
	if s.Backend == nil {
		return domain.SeverityScale{}, false, nil
	}
	values, err := s.Backend.CurrentOrganisation(ctx, domain.TypeSeverityScale)
	if err != nil || len(values) == 0 {
		return domain.SeverityScale{}, false, err
	}
	var scale domain.SeverityScale
	if err := json.Unmarshal(values[0].Payload, &scale); err != nil {
		return domain.SeverityScale{}, false, err
	}
	return scale, true, nil
}

// CheckSeverity refuses a value outside the confirmed scale (T-202).
func (s OrganisationService) CheckSeverity(ctx context.Context, value string) error {
	scale, found, err := s.SeverityScale(ctx)
	if err != nil {
		return err
	}
	if !found {
		return ErrSeverityScaleMissing
	}
	if !scale.Contains(value) {
		return errors.New("choose a severity from your organisation's scale")
	}
	return nil
}

// EmergencyConfiguration contains personal contact data; it is readable by
// Majors and Custodians only. Printing still needs the print permission.
func (s OrganisationService) EmergencyConfiguration(ctx context.Context, principal authz.Principal) (domain.EmergencyConfiguration, *domain.OrganisationView, error) {
	if s.Backend == nil {
		return domain.EmergencyConfiguration{}, nil, nil
	}
	if !principal.Authenticated || (principal.Role != authz.RoleMajor && !principal.Custodian) {
		return domain.EmergencyConfiguration{}, nil, ErrAuthorisationDenied
	}
	return s.emergencyConfiguration(ctx)
}

// emergencyConfiguration is used by the print service after it has checked
// print authority.
func (s OrganisationService) emergencyConfiguration(ctx context.Context) (domain.EmergencyConfiguration, *domain.OrganisationView, error) {
	values, err := s.Backend.CurrentOrganisation(ctx, domain.TypeEmergencyConfiguration)
	if err != nil || len(values) == 0 {
		return domain.EmergencyConfiguration{}, nil, err
	}
	var c domain.EmergencyConfiguration
	if err := json.Unmarshal(values[0].Payload, &c); err != nil {
		return domain.EmergencyConfiguration{}, nil, err
	}
	return c, &values[0], nil
}

// ChecklistTemplates returns confirmed, active templates to any Major.
func (s OrganisationService) ChecklistTemplates(ctx context.Context, principal authz.Principal) ([]domain.GovernedChecklistTemplate, error) {
	if s.Backend == nil {
		return nil, nil
	}
	if !principal.Authenticated || principal.HumanID == "" {
		return nil, ErrAuthorisationDenied
	}
	values, err := s.Backend.CurrentOrganisation(ctx, domain.TypeChecklistTemplate)
	if err != nil {
		return nil, err
	}
	var out []domain.GovernedChecklistTemplate
	for _, v := range values {
		var t domain.GovernedChecklistTemplate
		if err := json.Unmarshal(v.Payload, &t); err != nil {
			return nil, err
		}
		out = append(out, t)
	}
	return out, nil
}

// EmergencyForPrint returns the confirmed emergency configuration for the
// print service, which has already checked print authority.
func (s OrganisationService) EmergencyForPrint(ctx context.Context) (domain.EmergencyConfiguration, bool, error) {
	if s.Backend == nil {
		return domain.EmergencyConfiguration{}, false, nil
	}
	c, view, err := s.emergencyConfiguration(ctx)
	return c, view != nil, err
}

// EmergencyChangedAt reports when the emergency configuration was last
// confirmed, so pack freshness can be judged against it (T-204).
func (s OrganisationService) EmergencyChangedAt(ctx context.Context) (time.Time, error) {
	if s.Backend == nil {
		return time.Time{}, nil
	}
	values, err := s.Backend.CurrentOrganisation(ctx, domain.TypeEmergencyConfiguration)
	if err != nil || len(values) == 0 {
		return time.Time{}, err
	}
	return values[0].At, nil
}

// ProposeSinglePersonWindow proposes opening, changing or closing a window.
// A different Major must confirm it; a window can never approve itself.
func (s OrganisationService) ProposeSinglePersonWindow(ctx context.Context, principal authz.Principal, recordID string, w domain.SinglePersonWindow, now time.Time) (ledger.Record, error) {
	if err := s.requireMajor(principal); err != nil {
		return ledger.Record{}, err
	}
	if w.Active && s.MajorEligible != nil {
		for _, human := range w.Humans {
			ok, err := s.MajorEligible(ctx, human)
			if err != nil {
				return ledger.Record{}, err
			}
			if !ok {
				return ledger.Record{}, errors.New("only Majors can be named in a single-person window: " + human)
			}
		}
	}
	record, err := domain.ProposeSinglePersonWindow(recordID, principal.HumanID, now, w)
	return s.append(ctx, record, err)
}

// Windows returns every confirmed window, newest first.
func (s OrganisationService) Windows(ctx context.Context) ([]domain.SinglePersonWindow, []domain.OrganisationView, error) {
	if s.Backend == nil {
		return nil, nil, nil
	}
	values, err := s.Backend.CurrentOrganisation(ctx, domain.TypeSinglePersonWindow)
	if err != nil {
		return nil, nil, err
	}
	out := make([]domain.SinglePersonWindow, 0, len(values))
	for _, v := range values {
		var w domain.SinglePersonWindow
		if err := json.Unmarshal(v.Payload, &w); err != nil {
			return nil, nil, err
		}
		out = append(out, w)
	}
	return out, values, nil
}

// Permits implements WindowPolicy.
func (s OrganisationService) Permits(ctx context.Context, human, recordType string, at time.Time) (bool, error) {
	windows, _, err := s.Windows(ctx)
	if err != nil {
		return false, err
	}
	for _, w := range windows {
		if w.Covers(human, recordType, at) {
			return true, nil
		}
	}
	return false, nil
}

// OpenWindow reports whether any confirmed window is open at a time.
func (s OrganisationService) OpenWindow(ctx context.Context, at time.Time) (bool, error) {
	windows, _, err := s.Windows(ctx)
	if err != nil {
		return false, err
	}
	for _, w := range windows {
		if w.Active && !at.Before(w.StartsAt) && at.Before(w.EndsAt) {
			return true, nil
		}
	}
	return false, nil
}
