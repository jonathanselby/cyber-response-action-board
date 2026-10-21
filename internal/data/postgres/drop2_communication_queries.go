package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type PendingCommunicationView struct {
	IncidentID  string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  time.Time
}

func NewCommunicationStoreFromKernel(kernel *Kernel) (*CommunicationStore, error) {
	return NewCommunicationStore(kernel.pool)
}

func (s *CommunicationStore) PendingForIncident(ctx context.Context, incidentID, compartment string) ([]PendingCommunicationView, error) {
	query := `SELECT incident_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.communication_proposals WHERE incident_id=$1 AND compartment='GENERAL' ORDER BY proposed_at,record_type,record_id`
	args := []any{incidentID}
	if compartment != "" && compartment != "GENERAL" {
		query = `SELECT incident_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.communication_proposals WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY proposed_at,record_type,record_id`
		args = append(args, compartment)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingCommunicationView
	for rows.Next() {
		var value PendingCommunicationView
		if err := rows.Scan(&value.IncidentID, &value.RecordID, &value.RecordType, &value.Compartment, &value.Version, &value.Maker, &value.Mode, &value.ProposedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *CommunicationStore) Drafts(ctx context.Context, incidentID, compartment string) ([]domain.CommunicationDraftView, error) {
	query := `SELECT record_id,incident_id,mode,compartment,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,sha256,maker,recorded_at FROM crab.communication_drafts WHERE incident_id=$1 AND compartment='GENERAL' ORDER BY recorded_at DESC,record_id`
	args := []any{incidentID}
	if compartment != "" && compartment != "GENERAL" {
		query = `SELECT record_id,incident_id,mode,compartment,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,sha256,maker,recorded_at FROM crab.communication_drafts WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY recorded_at DESC,record_id`
		args = append(args, compartment)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.CommunicationDraftView
	for rows.Next() {
		var view domain.CommunicationDraftView
		var audience, channels, sources, related []byte
		if err := rows.Scan(&view.RecordID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Draft.Kind, &view.Draft.Subject, &view.Draft.Body, &audience, &channels, &sources, &related, &view.Draft.CadenceID, &view.SHA256, &view.Maker, &view.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(audience, &view.Draft.Audience); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(channels, &view.Draft.Channels); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(sources, &view.Draft.SourceRefs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(related, &view.Draft.RelatedRecordIDs); err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, rows.Err()
}

func (s *CommunicationStore) Issued(ctx context.Context, incidentID, compartment string) ([]domain.CommunicationView, error) {
	query := `SELECT record_id,incident_id,mode,compartment,draft_record_id,draft_sha256,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,maker,confirmer,issued_at FROM crab.communications WHERE incident_id=$1 AND compartment='GENERAL' ORDER BY issued_at DESC,record_id`
	args := []any{incidentID}
	if compartment != "" && compartment != "GENERAL" {
		query = `SELECT record_id,incident_id,mode,compartment,draft_record_id,draft_sha256,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,maker,confirmer,issued_at FROM crab.communications WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY issued_at DESC,record_id`
		args = append(args, compartment)
	}
	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []domain.CommunicationView
	for rows.Next() {
		var view domain.CommunicationView
		var audience, channels, sources, related []byte
		if err := rows.Scan(&view.RecordID, &view.IncidentID, &view.Mode, &view.Compartment, &view.Issue.DraftRecordID, &view.Issue.DraftSHA256, &view.Issue.Kind, &view.Issue.Subject, &view.Issue.Body, &audience, &channels, &sources, &related, &view.Issue.CadenceID, &view.Maker, &view.Confirmer, &view.IssuedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(audience, &view.Issue.Audience); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(channels, &view.Issue.Channels); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(sources, &view.Issue.SourceRefs); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(related, &view.Issue.RelatedRecordIDs); err != nil {
			return nil, err
		}
		result = append(result, view)
	}
	return result, rows.Err()
}

func (s *CommunicationStore) Cadences(ctx context.Context, incidentID, compartment string) ([]domain.CommunicationCadenceView, error) {
	query := `SELECT cadence_id,incident_id,mode,compartment,name,interval_minutes,first_due_at,owner_human_id,audience,channels,maker,confirmer,confirmed_at,last_generated_at,next_due_at,simulated FROM crab.communication_cadences WHERE incident_id=$1 AND compartment='GENERAL' ORDER BY next_due_at,cadence_id`
	args := []any{incidentID}
	if compartment != "" && compartment != "GENERAL" {
		query = `SELECT cadence_id,incident_id,mode,compartment,name,interval_minutes,first_due_at,owner_human_id,audience,channels,maker,confirmer,confirmed_at,last_generated_at,next_due_at,simulated FROM crab.communication_cadences WHERE incident_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY next_due_at,cadence_id`
		args = append(args, compartment)
	}
	rows, err := s.pool.Query(ctx, query, args...)
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
