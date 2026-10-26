package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

var (
	ErrCommunicationUnavailable        = errors.New("communication unavailable")
	ErrCommunicationNotDue             = errors.New("communication cadence is not due")
	ErrCommunicationChannelNotApproved = errors.New("notification channel is not approved for this communication")
)

type CommunicationBackend interface {
	AppendCommunication(context.Context, ledger.Record) error
	PendingCommunication(context.Context, string, string, string) (ledger.Record, bool, error)
	CommunicationDraft(context.Context, string, string) (domain.CommunicationDraftView, bool, error)
	IssuedCommunication(context.Context, string, string) (domain.CommunicationView, bool, error)
	CommunicationCadence(context.Context, string, string) (domain.CommunicationCadenceView, bool, error)
}

type IncidentPictureReader interface {
	CommonOperatingPicture(context.Context, authz.Principal, string, string) (domain.OperatingPicture, bool, error)
}

type NotificationChannel struct {
	Name          string
	Type          string
	Target        string
	DestinationID string
	External      bool
	Timeout       time.Duration
}

type NotificationMessage struct {
	CommunicationID string
	Mode            string
	Subject         string
	Body            string
	Audience        []string
}

type NotificationAttempt struct {
	Status          string
	Detail          string
	Acknowledgement string
	Duration        time.Duration
}

type NotificationSender interface {
	Deliver(context.Context, NotificationChannel, NotificationMessage) NotificationAttempt
}

type CommunicationService struct {
	Backend  CommunicationBackend
	Incident IncidentPictureReader
	Notifier NotificationSender
	Channels []NotificationChannel
	Window   WindowPolicy
}

func (s CommunicationService) RecordDraft(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.CommunicationDraft, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordCommunicationDraft(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCommunication(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s CommunicationService) ProposeIssue(ctx context.Context, principal authz.Principal, incidentID, draftRecordID, issueRecordID string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	draft, found, err := s.Backend.CommunicationDraft(ctx, incidentID, draftRecordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrCommunicationUnavailable
	}
	scope := domain.Scope{IncidentID: draft.IncidentID, Mode: draft.Mode, Compartment: draft.Compartment}
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	issue, err := domain.CommunicationIssueFromDraft(draft.RecordID, draft.SHA256, draft.Draft)
	if err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeCommunicationIssue(scope, issueRecordID, principal.HumanID, now, issue)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCommunication(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s CommunicationService) ProposeCadence(ctx context.Context, principal authz.Principal, scope domain.Scope, cadenceID string, value domain.CommunicationCadence, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionPropose, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.ProposeCommunicationCadence(scope, cadenceID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCommunication(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s CommunicationService) Decide(ctx context.Context, principal authz.Principal, incidentID, recordType, recordID, expectedVersion, outcome string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	proposal, found, err := s.Backend.PendingCommunication(ctx, incidentID, recordType, recordID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrProposalNotFound
	}
	scope := domain.Scope{IncidentID: proposal.IncidentID, Mode: proposal.IncidentMode, Compartment: proposal.Compartment}
	if err := s.authorise(principal, authz.ActionConfirm, scope); err != nil {
		return ledger.Record{}, err
	}
	decided, err := decideGoverned(ctx, s.Window, proposal, expectedVersion, principal, outcome, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCommunication(ctx, decided); err != nil {
		return ledger.Record{}, err
	}
	return decided, nil
}

func (s CommunicationService) RecordIncomingEnquiry(ctx context.Context, principal authz.Principal, scope domain.Scope, recordID string, value domain.IncomingEnquiry, now time.Time) (ledger.Record, error) {
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	record, err := domain.RecordIncomingEnquiry(scope, recordID, principal.HumanID, now, value)
	if err != nil {
		return ledger.Record{}, err
	}
	if err := s.Backend.AppendCommunication(ctx, record); err != nil {
		return ledger.Record{}, err
	}
	return record, nil
}

func (s CommunicationService) GenerateDueSituationReportDraft(ctx context.Context, principal authz.Principal, incidentID, cadenceID, draftRecordID string, now time.Time) (ledger.Record, error) {
	if err := s.ready(); err != nil {
		return ledger.Record{}, err
	}
	if s.Incident == nil {
		return ledger.Record{}, errors.New("incident operating-picture reader is required")
	}
	cadence, found, err := s.Backend.CommunicationCadence(ctx, incidentID, cadenceID)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrCommunicationUnavailable
	}
	scope := domain.Scope{IncidentID: cadence.IncidentID, Mode: cadence.Mode, Compartment: cadence.Compartment}
	if err := s.authorise(principal, authz.ActionRecord, scope); err != nil {
		return ledger.Record{}, err
	}
	if cadence.NextDueAt.IsZero() || now.Before(cadence.NextDueAt) {
		return ledger.Record{}, ErrCommunicationNotDue
	}
	picture, found, err := s.Incident.CommonOperatingPicture(ctx, principal, cadence.IncidentID, cadence.Compartment)
	if err != nil {
		return ledger.Record{}, err
	}
	if !found {
		return ledger.Record{}, ErrCommunicationUnavailable
	}
	draft, err := domain.BuildSituationReportDraft(picture, cadence.CadenceID, cadence.Cadence.Audience, cadence.Cadence.Channels, now)
	if err != nil {
		return ledger.Record{}, err
	}
	return s.RecordDraft(ctx, principal, scope, draftRecordID, draft, now)
}

func (s CommunicationService) NotifyIssued(ctx context.Context, principal authz.Principal, incidentID, communicationID string, requestedChannels []string, now time.Time) ([]domain.NotificationOutcome, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	communication, found, err := s.Backend.IssuedCommunication(ctx, incidentID, communicationID)
	if err != nil {
		return nil, err
	}
	if !found {
		return nil, ErrCommunicationUnavailable
	}
	scope := domain.Scope{IncidentID: communication.IncidentID, Mode: communication.Mode, Compartment: communication.Compartment}
	if err := s.authorise(principal, authz.ActionNotify, scope); err != nil {
		return nil, err
	}
	channels, err := approvedNotificationChannels(communication.Issue.Channels, requestedChannels)
	if err != nil {
		return nil, err
	}
	message := NotificationMessage{CommunicationID: communication.RecordID, Mode: communication.Mode, Subject: communication.Issue.Subject, Body: communication.Issue.Body, Audience: append([]string(nil), communication.Issue.Audience...)}
	outcomes := make([]domain.NotificationOutcome, 0, len(channels))
	for i, name := range channels {
		attemptedAt := now.UTC()
		channel, configured := s.channel(name)
		outcome := domain.NotificationOutcome{CommunicationID: communication.RecordID, Channel: name, AttemptedAt: attemptedAt}
		if !configured {
			outcome.Status = domain.NotificationFailed
			outcome.Detail = "channel not configured"
		} else {
			outcome.DestinationID = channel.DestinationID
			switch {
			case communication.Mode == "exercise" && channel.External:
				outcome.Status = domain.NotificationSuppressed
				outcome.Detail = "external channel suppressed in exercise mode"
			case s.Notifier == nil:
				outcome.Status = domain.NotificationFailed
				outcome.Detail = "notification adapter unavailable"
			default:
				attempt := s.Notifier.Deliver(ctx, channel, message)
				outcome.Status = attempt.Status
				outcome.Detail = attempt.Detail
				outcome.Acknowledgement = attempt.Acknowledgement
				outcome.DurationMS = attempt.Duration.Milliseconds()
				if outcome.Status != domain.NotificationDelivered && outcome.Status != domain.NotificationFailed {
					outcome.Status = domain.NotificationFailed
					outcome.Detail = "notification adapter returned invalid status"
					outcome.Acknowledgement = ""
				}
			}
		}
		recordID := notificationOutcomeRecordID(communication.RecordID, name, attemptedAt, i)
		record, recordErr := domain.RecordNotificationOutcome(scope, recordID, principal.HumanID, attemptedAt, outcome)
		if recordErr != nil {
			return outcomes, recordErr
		}
		if recordErr = s.Backend.AppendCommunication(ctx, record); recordErr != nil {
			return outcomes, recordErr
		}
		outcomes = append(outcomes, outcome)
	}
	return outcomes, nil
}

func (s CommunicationService) authorise(principal authz.Principal, action authz.Action, scope domain.Scope) error {
	if err := s.ready(); err != nil {
		return err
	}
	if d := authz.Authorise(principal, action, authz.ObjectScope{IncidentID: scope.IncidentID, Compartment: scope.Compartment}); !d.Allowed {
		return ErrAuthorisationDenied
	}
	return nil
}

func (s CommunicationService) ready() error {
	if s.Backend == nil {
		return errors.New("communication backend is required")
	}
	return nil
}

func (s CommunicationService) channel(name string) (NotificationChannel, bool) {
	for _, channel := range s.Channels {
		if channel.Name == name {
			return channel, true
		}
	}
	return NotificationChannel{}, false
}

func approvedNotificationChannels(approved, requested []string) ([]string, error) {
	if len(requested) == 0 {
		if len(approved) == 0 {
			return nil, errors.New("issued communication has no approved notification channels")
		}
		return append([]string(nil), approved...), nil
	}
	allowed := make(map[string]struct{}, len(approved))
	for _, name := range approved {
		allowed[name] = struct{}{}
	}
	seen := make(map[string]struct{}, len(requested))
	result := make([]string, 0, len(requested))
	for _, name := range requested {
		name = strings.TrimSpace(name)
		if _, ok := allowed[name]; !ok {
			return nil, ErrCommunicationChannelNotApproved
		}
		if _, ok := seen[name]; ok {
			return nil, errors.New("requested notification channels must be unique")
		}
		seen[name] = struct{}{}
		result = append(result, name)
	}
	return result, nil
}

func notificationOutcomeRecordID(communicationID, channel string, attemptedAt time.Time, index int) string {
	raw := fmt.Sprintf("%s|%s|%s|%d", communicationID, channel, attemptedAt.UTC().Format(time.RFC3339Nano), index)
	digest := sha256.Sum256([]byte(raw))
	return "notify-" + hex.EncodeToString(digest[:12])
}

func SortNotificationChannels(channels []NotificationChannel) {
	sort.Slice(channels, func(i, j int) bool { return channels[i].Name < channels[j].Name })
}
