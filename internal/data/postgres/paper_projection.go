package postgres

import (
	"context"
	"encoding/json"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// PaperProjection contributes restored paper-era events to the ordinary
// incident timeline. It owns no table and therefore has no independent reset:
// IncidentProjection resets the shared timeline before combined replay.
type PaperProjection struct{}

func (PaperProjection) Reset(context.Context, pgx.Tx) error { return nil }

func (PaperProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypePaperTranscription {
		return nil
	}
	if record.WriteSemantic != "record" || record.Status != "recorded" {
		return errors.New("paper transcription requires immediate record semantics")
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") || record.Compartment == "" {
		return errors.New("paper transcription requires incident scope")
	}
	value, err := domain.DecodePayload[domain.PaperTranscription](record)
	if err != nil {
		return err
	}
	if err := domain.ValidatePaperTranscription(value); err != nil {
		return err
	}
	awareness, err := json.Marshal([]domain.AwarenessTime{})
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.incident_timeline(sequence,incident_id,record_id,record_type,governance,compartment,summary,occurred_at,flagged_at,recorded_at,awareness,time_status) VALUES ($1,$2,$3,$4,'recorded',$5,$6,$7,NULL,$8,$9,$10)`, entry.Seq, record.IncidentID, record.RecordID, record.RecordType, record.Compartment, value.Statement, value.PaperOccurredAt.UTC(), record.Timestamp.UTC(), awareness, record.TimeStatus)
	return err
}
