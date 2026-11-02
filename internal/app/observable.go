package app

import (
	"context"
	"errors"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// ObservableBackend stores and reads observables (addendum A2).
type ObservableBackend interface {
	AppendObservable(context.Context, ledger.Record) error
	Current(context.Context, string) (domain.ObservableView, bool, error)
	Register(context.Context, string, string) ([]domain.ObservableView, error)
	History(context.Context, string) ([]domain.ObservableView, error)
	Sightings(ctx context.Context, incidentID string, kinds, values []string) (map[string][]domain.ObservableSighting, error)
}

// ObservableService records and lists observables. It never looks anything up
// outside CRAB.
type ObservableService struct{ Backend ObservableBackend }

// ObservableInput is a new observable.
type ObservableInput struct {
	Type, Value, Status, Source, TimeConfidence, Note string
	FirstSeen                                         *time.Time
}

// ObservableItem is one observable as a viewer sees it, with the other
// incidents holding the same value that this viewer may read.
type ObservableItem struct {
	View    domain.ObservableView
	Related []string
}

// Record records a new observable in scope.
func (s ObservableService) Record(ctx context.Context, principal authz.Principal, scope domain.Scope, observableID, recordID string, in ObservableInput, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("observable backend is required")
	}
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	normal, err := domain.NormaliseObservable(strings.TrimSpace(in.Type), in.Value)
	if err != nil {
		return ledger.Record{}, err
	}
	value := domain.Observable{ObservableID: observableID, Type: strings.TrimSpace(in.Type), Value: normal, Status: strings.TrimSpace(in.Status), Source: strings.TrimSpace(in.Source), Note: strings.TrimSpace(in.Note),
		FirstSeen: now, TimeConfidence: ledger.TimeTrusted}
	if in.FirstSeen != nil {
		// A stated time is not CRAB's governed time unless the person says so.
		value.FirstSeen = *in.FirstSeen
		value.TimeConfidence = strings.TrimSpace(in.TimeConfidence)
		if value.TimeConfidence == "" {
			value.TimeConfidence = ledger.TimeUntrusted
		}
	}
	record, err := domain.RecordObservable(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObservable(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// ChangeStatus records a new version of an observable with a new status.
// Its incident, compartment, type, value and first-seen time do not change.
func (s ObservableService) ChangeStatus(ctx context.Context, principal authz.Principal, observableID, recordID, status, source, note string, now time.Time) (ledger.Record, error) {
	if s.Backend == nil {
		return ledger.Record{}, errors.New("observable backend is required")
	}
	current, found, err := s.Backend.Current(ctx, observableID)
	if err != nil {
		return ledger.Record{}, err
	}
	// A hidden observable and a missing one are refused alike.
	if !found {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	if d := authz.Authorise(principal, authz.ActionRecord, authz.ObjectScope{IncidentID: current.IncidentID, Compartment: current.Compartment}); !d.Allowed {
		return ledger.Record{}, ErrAuthorisationDenied
	}
	scope := domain.Scope{IncidentID: current.IncidentID, Mode: current.Mode, Compartment: current.Compartment}
	record, err := domain.RecordObservable(scope, recordID, principal.HumanID, now, domain.NextObservableVersion(current.Observable, status, source, note))
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendObservable(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

// List returns the current observables the viewer may read in an incident's
// GENERAL compartment and one restricted compartment. A related incident is
// named only when the viewer may read that incident and that compartment;
// otherwise nothing about it is shown.
func (s ObservableService) List(ctx context.Context, principal authz.Principal, incidentID, compartment string) ([]ObservableItem, error) {
	if s.Backend == nil {
		return nil, errors.New("observable backend is required")
	}
	if compartment == "" {
		compartment = "GENERAL"
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}); !d.Allowed {
		return nil, ErrAuthorisationDenied
	}
	views, err := s.Backend.Register(ctx, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	kinds, values := make([]string, 0, len(views)), make([]string, 0, len(views))
	for _, v := range views {
		kinds, values = append(kinds, v.Observable.Type), append(values, v.Observable.Value)
	}
	sightings, err := s.Backend.Sightings(ctx, incidentID, kinds, values)
	if err != nil {
		return nil, err
	}
	out := make([]ObservableItem, 0, len(views))
	for _, v := range views {
		item := ObservableItem{View: v}
		seen := map[string]bool{}
		for _, sight := range sightings[v.Observable.Type+"\x00"+v.Observable.Value] {
			if seen[sight.IncidentID] {
				continue
			}
			if authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: sight.IncidentID, Compartment: sight.Compartment}).Allowed {
				seen[sight.IncidentID] = true
				item.Related = append(item.Related, sight.IncidentID)
			}
		}
		sort.Strings(item.Related)
		out = append(out, item)
	}
	return out, nil
}

// History returns every version of an observable the viewer may read.
func (s ObservableService) History(ctx context.Context, principal authz.Principal, observableID string) ([]domain.ObservableView, error) {
	current, found, err := s.Backend.Current(ctx, observableID)
	if err != nil {
		return nil, err
	}
	if !found || !authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: current.IncidentID, Compartment: current.Compartment}).Allowed {
		return nil, ErrAuthorisationDenied
	}
	return s.Backend.History(ctx, observableID)
}

// ManagementBackend computes the management summary over given scope pairs.
type ManagementBackend interface {
	ManagementSummary(ctx context.Context, incidents, compartments []string, now time.Time) (domain.ManagementSummary, error)
}

// ManagementService builds the Command Summary management panel (addendum
// A3) over exactly the incidents and compartments the viewer may read.
type ManagementService struct{ Backend ManagementBackend }

// Summary returns the five fixed metrics for this viewer.
func (s ManagementService) Summary(ctx context.Context, principal authz.Principal, now time.Time) (domain.ManagementSummary, error) {
	if s.Backend == nil {
		return domain.ManagementSummary{}, errors.New("management backend is required")
	}
	if !principal.Authenticated || principal.HumanID == "" {
		return domain.ManagementSummary{}, ErrAuthorisationDenied
	}
	var incidents, compartments []string
	for incidentID, allowed := range principal.Incidents {
		if !allowed {
			continue
		}
		for compartment := range principal.Compartments[incidentID] {
			if authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: incidentID, Compartment: compartment}).Allowed {
				incidents, compartments = append(incidents, incidentID), append(compartments, compartment)
			}
		}
	}
	return s.Backend.ManagementSummary(ctx, incidents, compartments, now)
}
