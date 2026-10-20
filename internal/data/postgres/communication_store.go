package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type CommunicationStore struct{ pool *pgxpool.Pool }

func NewCommunicationStore(pool *pgxpool.Pool) (*CommunicationStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &CommunicationStore{pool: pool}, nil
}

func (s *CommunicationStore) CommunicationDraft(ctx context.Context, incidentID, recordID string) (domain.CommunicationDraftView, bool, error) {
	var view domain.CommunicationDraftView
	var audience, channels, sources, related []byte
	err := s.pool.QueryRow(ctx, `SELECT record_id,incident_id,mode,compartment,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,sha256,maker,recorded_at FROM crab.communication_drafts WHERE incident_id=$1 AND record_id=$2`, incidentID, recordID).Scan(&view.RecordID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Draft.Kind, &view.Draft.Subject, &view.Draft.Body, &audience, &channels, &sources, &related, &view.Draft.CadenceID, &view.SHA256, &view.Maker, &view.RecordedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return view, false, nil
	}
	if err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(audience, &view.Draft.Audience); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(channels, &view.Draft.Channels); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(sources, &view.Draft.SourceRefs); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(related, &view.Draft.RelatedRecordIDs); err != nil {
		return view, false, err
	}
	return view, true, domain.ValidateCommunicationDraft(view.Draft)
}

func (s *CommunicationStore) IssuedCommunication(ctx context.Context, incidentID, recordID string) (domain.CommunicationView, bool, error) {
	var view domain.CommunicationView
	var audience, channels, sources, related []byte
	err := s.pool.QueryRow(ctx, `SELECT record_id,incident_id,mode,compartment,draft_record_id,draft_sha256,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,maker,confirmer,issued_at FROM crab.communications WHERE incident_id=$1 AND record_id=$2`, incidentID, recordID).Scan(&view.RecordID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Issue.DraftRecordID, &view.Issue.DraftSHA256, &view.Issue.Kind, &view.Issue.Subject, &view.Issue.Body, &audience, &channels, &sources, &related, &view.Issue.CadenceID, &view.Maker, &view.Confirmer, &view.IssuedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return view, false, nil
	}
	if err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(audience, &view.Issue.Audience); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(channels, &view.Issue.Channels); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(sources, &view.Issue.SourceRefs); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(related, &view.Issue.RelatedRecordIDs); err != nil {
		return view, false, err
	}
	return view, true, domain.ValidateCommunicationIssue(view.Issue)
}

func (s *CommunicationStore) CommunicationCadence(ctx context.Context, incidentID, cadenceID string) (domain.CommunicationCadenceView, bool, error) {
	var view domain.CommunicationCadenceView
	var audience, channels []byte
	err := s.pool.QueryRow(ctx, `SELECT cadence_id,incident_id,mode,compartment,name,interval_minutes,first_due_at,owner_human_id,audience,channels,maker,confirmer,confirmed_at,last_generated_at,next_due_at,simulated FROM crab.communication_cadences WHERE incident_id=$1 AND cadence_id=$2`, incidentID, cadenceID).Scan(&view.CadenceID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Cadence.Name, &view.Cadence.IntervalMinutes, &view.Cadence.FirstDueAt, &view.Cadence.OwnerHumanID, &audience, &channels, &view.Maker, &view.Confirmer, &view.ConfirmedAt, &view.LastGeneratedAt, &view.NextDueAt, &view.Simulated)
	if errors.Is(err, pgx.ErrNoRows) {
		return view, false, nil
	}
	if err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(audience, &view.Cadence.Audience); err != nil {
		return view, false, err
	}
	if err := json.Unmarshal(channels, &view.Cadence.Channels); err != nil {
		return view, false, err
	}
	return view, true, domain.ValidateCommunicationCadence(view.Cadence)
}

func (s *CommunicationStore) NotificationOutcomes(ctx context.Context, incidentID, communicationID string) ([]domain.NotificationOutcomeView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,incident_id,compartment,communication_id,channel,destination_id,attempted_at,status,detail,acknowledgement,duration_ms,maker,recorded_at FROM crab.notification_outcomes WHERE incident_id=$1 AND communication_id=$2 ORDER BY attempted_at,record_id`, incidentID, communicationID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.NotificationOutcomeView
	for rows.Next() {
		var view domain.NotificationOutcomeView
		if err := rows.Scan(&view.RecordID, &view.IncidentID, &view.Compartment, &view.Outcome.CommunicationID, &view.Outcome.Channel, &view.Outcome.DestinationID, &view.Outcome.AttemptedAt, &view.Outcome.Status, &view.Outcome.Detail, &view.Outcome.Acknowledgement, &view.Outcome.DurationMS, &view.Maker, &view.RecordedAt); err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, rows.Err()
}

func (s *CommunicationStore) IncomingEnquiries(ctx context.Context, incidentID, compartment string) ([]domain.IncomingEnquiryView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,incident_id,compartment,source,contact,subject,message,received_at,assigned_to_human_id,related_record_ids,maker,recorded_at FROM crab.incoming_enquiries WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY received_at,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.IncomingEnquiryView
	for rows.Next() {
		var view domain.IncomingEnquiryView
		var related []byte
		if err := rows.Scan(&view.RecordID, &view.IncidentID, &view.Compartment, &view.Enquiry.Source, &view.Enquiry.Contact, &view.Enquiry.Subject, &view.Enquiry.Message, &view.Enquiry.ReceivedAt, &view.Enquiry.AssignedToHumanID, &related, &view.Maker, &view.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(related, &view.Enquiry.RelatedRecordIDs); err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, rows.Err()
}

func (s *CommunicationStore) DueCommunicationCadences(ctx context.Context, now time.Time, limit int) ([]domain.CommunicationCadenceView, error) {
	if limit <= 0 || limit > 1000 {
		return nil, errors.New("communication cadence limit must be between 1 and 1000")
	}
	rows, err := s.pool.Query(ctx, `SELECT cadence_id,incident_id,mode,compartment,name,interval_minutes,first_due_at,owner_human_id,audience,channels,maker,confirmer,confirmed_at,last_generated_at,next_due_at,simulated FROM crab.communication_cadences WHERE next_due_at <= $1 ORDER BY next_due_at,incident_id,cadence_id LIMIT $2`, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.CommunicationCadenceView
	for rows.Next() {
		var view domain.CommunicationCadenceView
		var audience, channels []byte
		if err := rows.Scan(&view.CadenceID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Cadence.Name, &view.Cadence.IntervalMinutes, &view.Cadence.FirstDueAt, &view.Cadence.OwnerHumanID, &audience, &channels, &view.Maker, &view.Confirmer, &view.ConfirmedAt, &view.LastGeneratedAt, &view.NextDueAt, &view.Simulated); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(audience, &view.Cadence.Audience); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(channels, &view.Cadence.Channels); err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, rows.Err()
}
