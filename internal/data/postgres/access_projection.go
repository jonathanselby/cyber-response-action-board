package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type AccessProjection struct{}

func (AccessProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.output_permissions, crab.compartment_memberships, crab.incident_memberships, crab.custodian_designations, crab.role_eligibility`)
	return err
}

func (AccessProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.WriteSemantic != "governed" || record.Status != "confirmed" {
		return nil
	}
	change, recognised, err := access.Decode(record.RecordType, record.Data)
	if err != nil || !recognised {
		return err
	}
	if err := validateAccessEnvelope(record, change); err != nil {
		return err
	}

	switch change.RecordType {
	case access.RecordRoleEligibility:
		if change.Active {
			_, err = tx.Exec(ctx, `INSERT INTO crab.role_eligibility(human_id, role) VALUES ($1,$2) ON CONFLICT (human_id) DO UPDATE SET role=EXCLUDED.role`, change.HumanID, string(change.Role))
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.role_eligibility WHERE human_id=$1`, change.HumanID)
		}
	case access.RecordCustodianDesignation:
		if change.Active {
			_, err = tx.Exec(ctx, `INSERT INTO crab.custodian_designations(human_id) VALUES ($1) ON CONFLICT DO NOTHING`, change.HumanID)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.custodian_designations WHERE human_id=$1`, change.HumanID)
		}
	case access.RecordIncidentMembership:
		if change.Active {
			if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_memberships(human_id, incident_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`, change.HumanID, change.IncidentID); err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO crab.compartment_memberships(human_id, incident_id, compartment) VALUES ($1,$2,'GENERAL') ON CONFLICT DO NOTHING`, change.HumanID, change.IncidentID)
			}
		} else {
			if _, err = tx.Exec(ctx, `DELETE FROM crab.compartment_memberships WHERE human_id=$1 AND incident_id=$2`, change.HumanID, change.IncidentID); err == nil {
				_, err = tx.Exec(ctx, `DELETE FROM crab.incident_memberships WHERE human_id=$1 AND incident_id=$2`, change.HumanID, change.IncidentID)
			}
		}
	case access.RecordCompartmentMembership:
		if change.Active {
			var member bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_memberships WHERE human_id=$1 AND incident_id=$2)`, change.HumanID, change.IncidentID).Scan(&member); err == nil && !member {
				err = errors.New("restricted compartment read-in requires active incident membership")
			}
			if err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO crab.compartment_memberships(human_id, incident_id, compartment) VALUES ($1,$2,$3) ON CONFLICT DO NOTHING`, change.HumanID, change.IncidentID, change.Compartment)
			}
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.compartment_memberships WHERE human_id=$1 AND incident_id=$2 AND compartment=$3`, change.HumanID, change.IncidentID, change.Compartment)
		}
	case access.RecordOutputPermission:
		if change.Active {
			_, err = tx.Exec(ctx, `INSERT INTO crab.output_permissions(human_id, action) VALUES ($1,$2) ON CONFLICT DO NOTHING`, change.HumanID, string(change.Action))
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.output_permissions WHERE human_id=$1 AND action=$2`, change.HumanID, string(change.Action))
		}
	}
	if err != nil {
		return fmt.Errorf("apply access projection: %w", err)
	}
	return nil
}

func validateAccessEnvelope(record ledger.Record, change access.Change) error {
	switch change.RecordType {
	case access.RecordRoleEligibility, access.RecordCustodianDesignation, access.RecordOutputPermission:
		if record.IncidentMode != "system" || record.IncidentID != "" {
			return errors.New("system access metadata requires system ledger scope")
		}
	case access.RecordIncidentMembership, access.RecordCompartmentMembership:
		if record.IncidentID != change.IncidentID || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
			return errors.New("incident access change envelope does not match authoritative incident scope")
		}
	}
	return nil
}

type AccessStore struct {
	pool *pgxpool.Pool
}

func NewAccessStore(pool *pgxpool.Pool) (*AccessStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &AccessStore{pool: pool}, nil
}

func (s *AccessStore) Principal(ctx context.Context, humanID string, authenticated bool) (authz.Principal, error) {
	principal := authz.Principal{
		Authenticated: authenticated,
		HumanID:       humanID,
		Incidents:     map[string]bool{},
		Compartments:  map[string]map[string]bool{},
		Outputs:       map[authz.Action]bool{},
	}
	var role string
	err := s.pool.QueryRow(ctx, `SELECT role FROM crab.role_eligibility WHERE human_id=$1`, humanID).Scan(&role)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return authz.Principal{}, fmt.Errorf("read role eligibility: %w", err)
	}
	if err == nil {
		principal.Role = authz.Role(role)
	}
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.custodian_designations WHERE human_id=$1)`, humanID).Scan(&principal.Custodian); err != nil {
		return authz.Principal{}, fmt.Errorf("read Custodian designation: %w", err)
	}

	rows, err := s.pool.Query(ctx, `SELECT incident_id FROM crab.incident_memberships WHERE human_id=$1`, humanID)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("read incident memberships: %w", err)
	}
	for rows.Next() {
		var incidentID string
		if err := rows.Scan(&incidentID); err != nil {
			rows.Close()
			return authz.Principal{}, err
		}
		principal.Incidents[incidentID] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return authz.Principal{}, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT incident_id, compartment FROM crab.compartment_memberships WHERE human_id=$1`, humanID)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("read compartment memberships: %w", err)
	}
	for rows.Next() {
		var incidentID, compartment string
		if err := rows.Scan(&incidentID, &compartment); err != nil {
			rows.Close()
			return authz.Principal{}, err
		}
		if principal.Compartments[incidentID] == nil {
			principal.Compartments[incidentID] = map[string]bool{}
		}
		principal.Compartments[incidentID][compartment] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return authz.Principal{}, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT action FROM crab.output_permissions WHERE human_id=$1`, humanID)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("read output permissions: %w", err)
	}
	for rows.Next() {
		var action string
		if err := rows.Scan(&action); err != nil {
			rows.Close()
			return authz.Principal{}, err
		}
		principal.Outputs[authz.Action(action)] = true
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return authz.Principal{}, err
	}
	rows.Close()
	return principal, nil
}

// MarshalAccessData is used by typed application commands/tests to avoid
// hand-building generic JSON at the presentation boundary.
func MarshalAccessData(v any) (json.RawMessage, error) {
	return json.Marshal(v)
}
