package app

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"github.com/jonathanselby/cyber-response-action-board/internal/proof"
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
	ErrNothingToResend                 = errors.New("nothing to resend: no channel's latest attempt failed")
)

type CommunicationBackend interface {
	AppendCommunication(context.Context, ledger.Record) error
	PendingCommunication(context.Context, string, string, string) (ledger.Record, bool, error)
	CommunicationDraft(context.Context, string, string) (domain.CommunicationDraftView, bool, error)
	IssuedCommunication(context.Context, string, string) (domain.CommunicationView, bool, error)
	CommunicationCadence(context.Context, string, string) (domain.CommunicationCadenceView, bool, error)
	NotificationOutcomes(context.Context, string, string) ([]domain.NotificationOutcomeView, error)
	NotificationRequests(context.Context, string, string) ([]domain.NotificationRequestView, error)
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
	// PerRecipient channels (email, SMS) deliver to a person's route; they
	// carry notices only, never communications.
	PerRecipient bool
	Timeout      time.Duration
	// Spec is the adapter's own channel configuration, opaque here.
	Spec any
}

type NotificationMessage struct {
	CommunicationID string
	Mode            string
	Subject         string
	Body            string
	Audience        []string
	// To and Ref address a per-recipient channel. To is never recorded.
	To, Ref string
}

type NotificationAttempt struct {
	Status          string
	Detail          string
	Acknowledgement string
	Duration        time.Duration
}

type NotificationSender interface {
	Deliver(context.Context, NotificationChannel, NotificationMessage) NotificationAttempt
	// AcceptsAddress reports whether a per-recipient channel accepts an address.
	AcceptsAddress(NotificationChannel, string) bool
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
	communication, scope, err := s.issuedForNotify(ctx, principal, incidentID, communicationID)
	if err != nil {
		return nil, err
	}
	channels, err := approvedNotificationChannels(communication.Issue.Channels, requestedChannels)
	if err != nil {
		return nil, err
	}
	return s.deliver(ctx, principal, scope, communication, channels, nil, now)
}

// ResendFailed sends an issued communication again on the channels whose
// latest attempt failed, with the same approved content (T-234). It is never
// automatic: a person with notify permission asks for it.
func (s CommunicationService) ResendFailed(ctx context.Context, principal authz.Principal, incidentID, communicationID string, now time.Time) ([]domain.NotificationOutcome, error) {
	communication, scope, err := s.issuedForNotify(ctx, principal, incidentID, communicationID)
	if err != nil {
		return nil, err
	}
	outcomes, err := s.Backend.NotificationOutcomes(ctx, communication.IncidentID, communication.RecordID)
	if err != nil {
		return nil, err
	}
	failed, attempts := FailedChannels(outcomes)
	if len(failed) == 0 {
		return nil, ErrNothingToResend
	}
	return s.deliver(ctx, principal, scope, communication, failed, attempts, now)
}

// FailedChannels returns, in channel order, the channels whose latest attempt
// failed and those attempts' record IDs.
func FailedChannels(outcomes []domain.NotificationOutcomeView) ([]string, []string) {
	latest := map[string]domain.NotificationOutcomeView{}
	for _, o := range outcomes { // ordered by attempt time
		latest[o.Outcome.Channel] = o
	}
	var channels, attempts []string
	for channel, o := range latest {
		if o.Outcome.Status == domain.NotificationFailed {
			channels = append(channels, channel)
		}
	}
	sort.Strings(channels)
	for _, channel := range channels {
		attempts = append(attempts, latest[channel].RecordID)
	}
	return channels, attempts
}

func (s CommunicationService) issuedForNotify(ctx context.Context, principal authz.Principal, incidentID, communicationID string) (domain.CommunicationView, domain.Scope, error) {
	if err := s.ready(); err != nil {
		return domain.CommunicationView{}, domain.Scope{}, err
	}
	communication, found, err := s.Backend.IssuedCommunication(ctx, incidentID, communicationID)
	if err != nil {
		return domain.CommunicationView{}, domain.Scope{}, err
	}
	if !found {
		return domain.CommunicationView{}, domain.Scope{}, ErrCommunicationUnavailable
	}
	scope := domain.Scope{IncidentID: communication.IncidentID, Mode: communication.Mode, Compartment: communication.Compartment}
	if err := s.authorise(principal, authz.ActionNotify, scope); err != nil {
		return domain.CommunicationView{}, domain.Scope{}, err
	}
	// The approved content must be exactly what was confirmed.
	if err := domain.ValidateCommunicationIssue(communication.Issue); err != nil {
		return domain.CommunicationView{}, domain.Scope{}, err
	}
	return communication, scope, nil
}

// deliver records the request, then attempts each channel in order and records
// every outcome. A request recorded before a crash stays visible with no
// attempts after restart (R-12, T-233).
func (s CommunicationService) deliver(ctx context.Context, principal authz.Principal, scope domain.Scope, communication domain.CommunicationView, channels, resendOf []string, now time.Time) ([]domain.NotificationOutcome, error) {
	requestID := notificationRequestRecordID(communication.RecordID, now, resendOf)
	request, err := domain.RecordNotificationRequest(scope, requestID, principal.HumanID, now, domain.NotificationRequest{CommunicationID: communication.RecordID, Channels: channels, ResendOf: resendOf})
	if err != nil {
		return nil, err
	}
	if err := s.Backend.AppendCommunication(ctx, request); err != nil {
		return nil, err
	}
	message := NotificationMessage{CommunicationID: communication.RecordID, Mode: communication.Mode, Subject: communication.Issue.Subject, Body: communication.Issue.Body, Audience: append([]string(nil), communication.Issue.Audience...)}
	outcomes := make([]domain.NotificationOutcome, 0, len(channels))
	proof.Hit("notification-before-attempt")
	for i, name := range channels {
		attemptedAt := now.UTC()
		channel, configured := s.channel(name)
		outcome := domain.NotificationOutcome{CommunicationID: communication.RecordID, Channel: name, AttemptedAt: attemptedAt, RequestID: requestID}
		switch {
		case !configured:
			outcome.Status = domain.NotificationFailed
			outcome.Detail = "channel not configured"
		case channel.PerRecipient:
			outcome.DestinationID = channel.DestinationID
			outcome.Status = domain.NotificationFailed
			outcome.Detail = "per-person channel; used for notices only"
		case communication.Mode == "exercise" && channel.External:
			outcome.DestinationID = channel.DestinationID
			outcome.Status = domain.NotificationSuppressed
			outcome.Detail = "external channel suppressed in exercise mode"
		case s.Notifier == nil:
			outcome.DestinationID = channel.DestinationID
			outcome.Status = domain.NotificationFailed
			outcome.Detail = "notification adapter unavailable"
		default:
			outcome.DestinationID = channel.DestinationID
			attempt := s.Notifier.Deliver(ctx, channel, message)
			outcome.Status = attempt.Status
			outcome.Detail = attempt.Detail
			outcome.Acknowledgement = attempt.Acknowledgement
			outcome.DurationMS = attempt.Duration.Milliseconds()
			if outcome.Status != domain.NotificationDelivered && outcome.Status != domain.NotificationFailed && outcome.Status != domain.NotificationSuppressed {
				outcome.Status = domain.NotificationFailed
				outcome.Detail = "notification adapter returned invalid status"
				outcome.Acknowledgement = ""
			}
		}
		recordID := notificationOutcomeRecordID(communication.RecordID, name, attemptedAt, i)
		if len(resendOf) > 0 {
			recordID = notificationOutcomeRecordID(requestID, name, attemptedAt, i)
		}
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

func notificationRequestRecordID(communicationID string, now time.Time, resendOf []string) string {
	raw := fmt.Sprintf("request|%s|%s|%s", communicationID, now.UTC().Format(time.RFC3339Nano), strings.Join(resendOf, ","))
	digest := sha256.Sum256([]byte(raw))
	return "notify-request-" + hex.EncodeToString(digest[:12])
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

// DeliveryAttempt is one recorded attempt, worded for people.
type DeliveryAttempt struct {
	RecordID, Channel, Status, StatusText, Detail string
	At                                            time.Time
}

// DeliveryRequest is one request to send and what happened to it. Missing
// lists requested channels with no recorded attempt: the process stopped
// before trying them.
type DeliveryRequest struct {
	RecordID, By string
	At           time.Time
	Resend       bool
	Attempts     []DeliveryAttempt
	Missing      []string
}

// DeliveryView is a communication's delivery history (T-233).
type DeliveryView struct {
	Requests []DeliveryRequest
	// Earlier holds attempts recorded before request records existed.
	Earlier   []DeliveryAttempt
	Failed    []string
	CanResend bool
}

// DeliveryStatusText words an outcome. A failure is never retried
// automatically and the screen says so.
func DeliveryStatusText(status string) string {
	switch status {
	case domain.NotificationDelivered:
		return "Delivered — accepted by the channel"
	case domain.NotificationFailed:
		return "Failed — not retried automatically"
	case domain.NotificationSuppressed:
		return "Suppressed — exercise, nothing sent"
	}
	return status
}

// Delivery builds a communication's delivery history for a person.
func (s CommunicationService) Delivery(ctx context.Context, principal authz.Principal, communication domain.CommunicationView) (DeliveryView, error) {
	var view DeliveryView
	if err := s.ready(); err != nil {
		return view, err
	}
	scope := authz.ObjectScope{IncidentID: communication.IncidentID, Compartment: communication.Compartment}
	if !authz.Authorise(principal, authz.ActionRead, scope).Allowed {
		return view, ErrAuthorisationDenied
	}
	requests, err := s.Backend.NotificationRequests(ctx, communication.IncidentID, communication.RecordID)
	if err != nil {
		return view, err
	}
	outcomes, err := s.Backend.NotificationOutcomes(ctx, communication.IncidentID, communication.RecordID)
	if err != nil {
		return view, err
	}
	byRequest := map[string][]DeliveryAttempt{}
	for _, o := range outcomes {
		a := DeliveryAttempt{RecordID: o.RecordID, Channel: o.Outcome.Channel, Status: o.Outcome.Status, StatusText: DeliveryStatusText(o.Outcome.Status), Detail: o.Outcome.Detail, At: o.Outcome.AttemptedAt}
		if o.Outcome.RequestID == "" {
			view.Earlier = append(view.Earlier, a)
			continue
		}
		byRequest[o.Outcome.RequestID] = append(byRequest[o.Outcome.RequestID], a)
	}
	for _, r := range requests {
		req := DeliveryRequest{RecordID: r.RecordID, By: r.Maker, At: r.RequestedAt, Resend: len(r.Request.ResendOf) > 0, Attempts: byRequest[r.RecordID]}
		tried := map[string]bool{}
		for _, a := range req.Attempts {
			tried[a.Channel] = true
		}
		for _, c := range r.Request.Channels {
			if !tried[c] {
				req.Missing = append(req.Missing, c)
			}
		}
		view.Requests = append(view.Requests, req)
	}
	view.Failed, _ = FailedChannels(outcomes)
	view.CanResend = len(view.Failed) > 0 && authz.Authorise(principal, authz.ActionNotify, scope).Allowed
	return view, nil
}
