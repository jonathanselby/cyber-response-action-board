package postgres

import (
	"context"
	"errors"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// DeclarationAccessProjection derives the declaration maker's initial mission
// access from the confirmed declaration itself. AccessProjection owns/reset the
// access tables; this projection only restores the frozen declaration invariant
// during append or a combined access+incident rebuild.
type DeclarationAccessProjection struct{}

func (DeclarationAccessProjection) Reset(context.Context, pgx.Tx) error { return nil }

func (DeclarationAccessProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeIncidentDeclaration || record.Status != "confirmed" {
		return nil
	}
	if record.WriteSemantic != "governed" || record.Maker == "" || record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
		return errors.New("confirmed incident declaration has invalid access scope")
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_sponsors(human_id,incident_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, record.Maker, record.IncidentID); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.incident_memberships(human_id,incident_id,effective_at,expires_at,reason) VALUES($1,$2,$3,NULL,'incident declaration') ON CONFLICT(human_id,incident_id) DO UPDATE SET effective_at=LEAST(crab.incident_memberships.effective_at,EXCLUDED.effective_at),expires_at=NULL,reason=EXCLUDED.reason`, record.Maker, record.IncidentID, record.Timestamp); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.compartment_memberships(human_id,incident_id,compartment,effective_at,expires_at,reason) VALUES($1,$2,'GENERAL',$3,NULL,'incident declaration') ON CONFLICT(human_id,incident_id,compartment) DO UPDATE SET effective_at=LEAST(crab.compartment_memberships.effective_at,EXCLUDED.effective_at),expires_at=NULL,reason=EXCLUDED.reason`, record.Maker, record.IncidentID, record.Timestamp)
	return err
}
