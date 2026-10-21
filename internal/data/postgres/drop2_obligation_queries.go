package postgres

import (
	"context"
	"encoding/json"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
)

type ObligationDefinitionView struct {
	DefinitionID string
	Definition   domain.ObligationDefinition
	Maker        string
	Confirmer    string
	ConfirmedAt  time.Time
}

type PendingObligationView struct {
	ScopeID     string
	RecordID    string
	RecordType  string
	Compartment string
	Version     string
	Maker       string
	Mode        string
	ProposedAt  time.Time
}

func NewObligationRuntimeFromKernel(kernel *Kernel, signer Signer) (*ObligationRuntime, *ObligationStore, error) {
	store, err := NewObligationStore(kernel.pool)
	if err != nil {
		return nil, nil, err
	}
	runtime, err := NewObligationRuntime(kernel, signer, store)
	if err != nil {
		return nil, nil, err
	}
	return runtime, store, nil
}

func (s *ObligationStore) Definitions(ctx context.Context) ([]ObligationDefinitionView, error) {
	rows, err := s.pool.Query(ctx, `SELECT definition_id,name,trigger_field,duration,unit,calendar,applicability_condition,recipient,owner_text,authority_source,warning_before_minutes,maker,confirmer,confirmed_at FROM crab.obligation_definitions ORDER BY name,definition_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []ObligationDefinitionView
	for rows.Next() {
		var value ObligationDefinitionView
		var calendar, warnings []byte
		if err := rows.Scan(&value.DefinitionID, &value.Definition.Name, &value.Definition.TriggerField, &value.Definition.Duration, &value.Definition.Unit, &calendar, &value.Definition.ApplicabilityCondition, &value.Definition.Recipient, &value.Definition.Owner, &value.Definition.AuthoritySource, &warnings, &value.Maker, &value.Confirmer, &value.ConfirmedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(calendar, &value.Definition.Calendar); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(warnings, &value.Definition.WarningBeforeMinutes); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}

func (s *ObligationStore) PendingForScope(ctx context.Context, scopeID, compartment string) ([]PendingObligationView, error) {
	var rows interface {
		Next() bool
		Scan(...any) error
		Close()
		Err() error
	}
	var err error
	if scopeID == "" {
		rows, err = s.pool.Query(ctx, `SELECT scope_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.obligation_proposals WHERE scope_id='' ORDER BY proposed_at,record_type,record_id`)
	} else if compartment == "GENERAL" || compartment == "" {
		rows, err = s.pool.Query(ctx, `SELECT scope_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.obligation_proposals WHERE scope_id=$1 AND compartment='GENERAL' ORDER BY proposed_at,record_type,record_id`, scopeID)
	} else {
		rows, err = s.pool.Query(ctx, `SELECT scope_id,record_id,record_type,compartment,version,maker,mode,proposed_at FROM crab.obligation_proposals WHERE scope_id=$1 AND compartment IN ('GENERAL',$2) ORDER BY proposed_at,record_type,record_id`, scopeID, compartment)
	}
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var result []PendingObligationView
	for rows.Next() {
		var value PendingObligationView
		if err := rows.Scan(&value.ScopeID, &value.RecordID, &value.RecordType, &value.Compartment, &value.Version, &value.Maker, &value.Mode, &value.ProposedAt); err != nil {
			return nil, err
		}
		result = append(result, value)
	}
	return result, rows.Err()
}
