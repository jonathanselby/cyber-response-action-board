package postgres

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type AccessProjection struct{}

func (AccessProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.access_proposals, crab.identities, crab.security_state, crab.output_permissions, crab.compartment_memberships, crab.incident_memberships, crab.compartment_controllers, crab.incident_sponsors, crab.custodian_designations, crab.role_eligibility`)
	return err
}

func (AccessProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !access.IsRecordType(record.RecordType) {
		return nil
	}
	if record.WriteSemantic != "governed" {
		return errors.New("access records require governed semantics")
	}

	if record.Status == "proposed" {
		data, err := domain.DecodeGovernedData(record.Data)
		if err != nil {
			return err
		}
		change, recognised, err := access.Decode(record.RecordType, data.Payload)
		if err != nil || !recognised {
			return err
		}
		if err := validateAccessEnvelope(record, change); err != nil {
			return err
		}
		scopeID := accessScopeID(record)
		_, err = tx.Exec(ctx, `INSERT INTO crab.access_proposals(scope_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode,time_status) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)`, scopeID, record.RecordID, record.RecordType, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode, record.TimeStatus)
		return err
	}

	if record.Status == "confirmed" || record.Status == "rejected" {
		data, governedErr := domain.DecodeGovernedData(record.Data)
		if governedErr == nil {
			scopeID := accessScopeID(record)
			var version, maker, compartment, mode, timeStatus string
			var payload []byte
			err := tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,time_status,payload FROM crab.access_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, record.RecordType, record.RecordID).Scan(&version, &maker, &compartment, &mode, &timeStatus, &payload)
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("access decision has no matching pending proposal")
			}
			if err != nil {
				return err
			}
			if version != data.Version || maker != record.Maker || compartment != record.Compartment || mode != record.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
				return errors.New("access decision does not match exact pending proposal")
			}
			if record.Status == "confirmed" {
				change, recognised, err := access.Decode(record.RecordType, data.Payload)
				if err != nil || !recognised {
					return err
				}
				if err := validateAccessEnvelope(record, change); err != nil {
					return err
				}
				if err := applyConfirmedAccess(ctx, tx, record, change); err != nil {
					return err
				}
			}
			_, err = tx.Exec(ctx, `DELETE FROM crab.access_proposals WHERE scope_id=$1 AND record_type=$2 AND record_id=$3`, scopeID, record.RecordType, record.RecordID)
			return err
		}

		// Slice 2 used direct confirmed fixtures before pending access proposals
		// existed. Keep replay compatibility for those historical ledger entries;
		// all Slice 6 application writes use the exact proposal/decision path above.
		if record.Status == "confirmed" {
			change, recognised, err := access.Decode(record.RecordType, record.Data)
			if err != nil || !recognised {
				return err
			}
			if err := validateAccessEnvelope(record, change); err != nil {
				return err
			}
			return applyConfirmedAccess(ctx, tx, record, change)
		}
		return governedErr
	}
	return errors.New("invalid access governance status")
}

func accessScopeID(record ledger.Record) string {
	if record.IncidentMode == "system" {
		return ""
	}
	return record.IncidentID
}

func applyConfirmedAccess(ctx context.Context, tx pgx.Tx, record ledger.Record, change access.Change) error {
	var err error
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
	case access.RecordIncidentSponsor:
		if change.Active {
			_, err = tx.Exec(ctx, `INSERT INTO crab.incident_sponsors(human_id,incident_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, change.HumanID, change.IncidentID)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.incident_sponsors WHERE human_id=$1 AND incident_id=$2`, change.HumanID, change.IncidentID)
		}
	case access.RecordCompartmentController:
		if change.Active {
			_, err = tx.Exec(ctx, `INSERT INTO crab.compartment_controllers(human_id,incident_id,compartment) VALUES($1,$2,$3) ON CONFLICT DO NOTHING`, change.HumanID, change.IncidentID, change.Compartment)
		} else {
			_, err = tx.Exec(ctx, `DELETE FROM crab.compartment_controllers WHERE human_id=$1 AND incident_id=$2 AND compartment=$3`, change.HumanID, change.IncidentID, change.Compartment)
		}
	case access.RecordIncidentMembership:
		if change.Active {
			effectiveAt := record.Timestamp
			if change.EffectiveAt != nil {
				effectiveAt = change.EffectiveAt.UTC()
			}
			var expiresAt any
			if change.ExpiresAt != nil {
				expiry := change.ExpiresAt.UTC()
				if !expiry.After(effectiveAt) {
					return errors.New("incident access expiry must be after effective time")
				}
				expiresAt = expiry
			}
			if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_memberships(human_id,incident_id,effective_at,expires_at,reason) VALUES($1,$2,$3,$4,$5) ON CONFLICT(human_id,incident_id) DO UPDATE SET effective_at=EXCLUDED.effective_at,expires_at=EXCLUDED.expires_at,reason=EXCLUDED.reason`, change.HumanID, change.IncidentID, effectiveAt, expiresAt, change.Reason); err == nil {
				_, err = tx.Exec(ctx, `INSERT INTO crab.compartment_memberships(human_id,incident_id,compartment,effective_at,expires_at,reason) VALUES($1,$2,'GENERAL',$3,$4,$5) ON CONFLICT(human_id,incident_id,compartment) DO UPDATE SET effective_at=EXCLUDED.effective_at,expires_at=EXCLUDED.expires_at,reason=EXCLUDED.reason`, change.HumanID, change.IncidentID, effectiveAt, expiresAt, change.Reason)
			}
		} else {
			if _, err = tx.Exec(ctx, `DELETE FROM crab.compartment_memberships WHERE human_id=$1 AND incident_id=$2`, change.HumanID, change.IncidentID); err == nil {
				_, err = tx.Exec(ctx, `DELETE FROM crab.incident_memberships WHERE human_id=$1 AND incident_id=$2`, change.HumanID, change.IncidentID)
			}
		}
	case access.RecordCompartmentMembership:
		if change.Active {
			var member bool
			if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_memberships WHERE human_id=$1 AND incident_id=$2 AND (effective_at IS NULL OR effective_at <= $3) AND (expires_at IS NULL OR expires_at > $3))`, change.HumanID, change.IncidentID, record.Timestamp).Scan(&member); err == nil && !member {
				err = errors.New("restricted compartment read-in requires active incident membership")
			}
			if err == nil {
				effectiveAt := record.Timestamp
				if change.EffectiveAt != nil {
					effectiveAt = change.EffectiveAt.UTC()
				}
				var expiresAt any
				if change.ExpiresAt != nil {
					expiry := change.ExpiresAt.UTC()
					if !expiry.After(effectiveAt) {
						return errors.New("compartment access expiry must be after effective time")
					}
					expiresAt = expiry
				}
				_, err = tx.Exec(ctx, `INSERT INTO crab.compartment_memberships(human_id,incident_id,compartment,effective_at,expires_at,reason) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT(human_id,incident_id,compartment) DO UPDATE SET effective_at=EXCLUDED.effective_at,expires_at=EXCLUDED.expires_at,reason=EXCLUDED.reason`, change.HumanID, change.IncidentID, change.Compartment, effectiveAt, expiresAt, change.Reason)
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
	case access.RecordIsolateMode:
		_, err = tx.Exec(ctx, `INSERT INTO crab.security_state(state_key, bool_value) VALUES ('isolate_mode',$1) ON CONFLICT (state_key) DO UPDATE SET bool_value=EXCLUDED.bool_value`, change.Active)
	case access.RecordIdentity:
		err = applyIdentity(ctx, tx, record, change)
	case access.RecordCredentialReset:
		// Audit only. Credential side effects happen in the application after
		// the decision commits, so replay never touches current credentials.
	}
	if err != nil {
		return fmt.Errorf("apply access projection: %w", err)
	}
	return nil
}

// applyIdentity projects a confirmed identity change. A disabled identity is
// final: replay refuses a later change, which the application never proposes.
// When a person stops being active, every session they hold is deleted in the
// same transaction. The account rule at principal load is the control; this is
// defence in depth and stops stale sessions accumulating.
func applyIdentity(ctx context.Context, tx pgx.Tx, record ledger.Record, change access.Change) error {
	id := change.Identity
	if id == nil {
		return errors.New("identity change payload missing")
	}
	var current string
	err := tx.QueryRow(ctx, `SELECT status FROM crab.identities WHERE human_id=$1`, id.HumanID).Scan(&current)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return err
	}
	if current == access.IdentityDisabled {
		return access.ErrIdentityDisabled
	}
	var expires any
	if id.AccountExpiresAt != nil {
		expires = id.AccountExpiresAt.UTC()
	}
	if _, err := tx.Exec(ctx, `INSERT INTO crab.identities(human_id,display_name,organisation,external,status,account_expires_at,reason,updated_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8)
		ON CONFLICT(human_id) DO UPDATE SET display_name=EXCLUDED.display_name, organisation=EXCLUDED.organisation, external=EXCLUDED.external,
			status=EXCLUDED.status, account_expires_at=EXCLUDED.account_expires_at, reason=EXCLUDED.reason, updated_at=EXCLUDED.updated_at`,
		id.HumanID, id.DisplayName, id.Organisation, id.External, id.Status, expires, id.Reason, record.Timestamp); err != nil {
		return err
	}
	if id.Status != access.IdentityActive {
		if _, err := tx.Exec(ctx, `DELETE FROM crab.sessions WHERE human_id=$1`, id.HumanID); err != nil {
			return err
		}
	}
	// Disabled is final, so credential material has no further use and is not
	// kept (Garage rule 8). Replaying this on rebuild is idempotent.
	if id.Status == access.IdentityDisabled {
		for _, stmt := range []string{`DELETE FROM crab.local_credentials WHERE human_id=$1`, `DELETE FROM crab.totp_credentials WHERE human_id=$1`} {
			if _, err := tx.Exec(ctx, stmt, id.HumanID); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateAccessEnvelope(record ledger.Record, change access.Change) error {
	switch change.RecordType {
	case access.RecordRoleEligibility, access.RecordCustodianDesignation, access.RecordOutputPermission, access.RecordIsolateMode, access.RecordIdentity, access.RecordCredentialReset:
		if record.IncidentMode != "system" || record.IncidentID != "" {
			return errors.New("system access metadata requires system ledger scope")
		}
	case access.RecordIncidentSponsor, access.RecordCompartmentController, access.RecordIncidentMembership, access.RecordCompartmentMembership:
		if record.IncidentID != change.IncidentID || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
			return errors.New("incident access change envelope does not match authoritative incident scope")
		}
	}
	if change.RecordType == access.RecordCompartmentController || change.RecordType == access.RecordCompartmentMembership {
		if record.Compartment != change.Compartment {
			return errors.New("compartment access change envelope does not match authoritative compartment")
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

func (s *AccessStore) IsolateMode(ctx context.Context) (bool, error) {
	var active bool
	err := s.pool.QueryRow(ctx, `SELECT bool_value FROM crab.security_state WHERE state_key='isolate_mode'`).Scan(&active)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("read isolate mode: %w", err)
	}
	return active, nil
}

func (s *AccessStore) PrincipalAt(ctx context.Context, humanID string, authenticated bool, now time.Time) (authz.Principal, error) {
	principal := authz.Principal{Authenticated: authenticated, HumanID: humanID, Incidents: map[string]bool{}, Compartments: map[string]map[string]bool{}, Outputs: map[authz.Action]bool{}}
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

	rows, err := s.pool.Query(ctx, `SELECT incident_id FROM crab.incident_memberships WHERE human_id=$1 AND (effective_at IS NULL OR effective_at <= $2) AND (expires_at IS NULL OR expires_at > $2)`, humanID, now)
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

	rows, err = s.pool.Query(ctx, `SELECT incident_id,compartment FROM crab.compartment_memberships WHERE human_id=$1 AND (effective_at IS NULL OR effective_at <= $2) AND (expires_at IS NULL OR expires_at > $2)`, humanID, now)
	if err != nil {
		return authz.Principal{}, fmt.Errorf("read compartment memberships: %w", err)
	}
	for rows.Next() {
		var incidentID, compartment string
		if err := rows.Scan(&incidentID, &compartment); err != nil {
			rows.Close()
			return authz.Principal{}, err
		}
		if !principal.Incidents[incidentID] {
			continue
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

// IdentityState returns the projected identity of one person and whether they
// are a legacy account (role eligibility or Custodian designation with no
// identity record). It reports state only; the account rule is access.Usable.
func (s *AccessStore) IdentityState(ctx context.Context, humanID string) (access.IdentityState, bool, error) {
	var state access.IdentityState
	var expires sql.NullTime
	err := s.pool.QueryRow(ctx, `SELECT display_name,organisation,external,status,account_expires_at FROM crab.identities WHERE human_id=$1`, humanID).
		Scan(&state.DisplayName, &state.Organisation, &state.External, &state.Status, &expires)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return access.IdentityState{}, false, fmt.Errorf("read identity: %w", err)
	}
	if err == nil {
		state.Found = true
		if expires.Valid {
			v := expires.Time.UTC()
			state.AccountExpiresAt = &v
		}
		return state, false, nil
	}
	var legacy bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.role_eligibility WHERE human_id=$1) OR EXISTS(SELECT 1 FROM crab.custodian_designations WHERE human_id=$1)`, humanID).Scan(&legacy); err != nil {
		return access.IdentityState{}, false, fmt.Errorf("read legacy eligibility: %w", err)
	}
	return state, legacy, nil
}

func (s *AccessStore) IsSponsor(ctx context.Context, humanID, incidentID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_sponsors WHERE human_id=$1 AND incident_id=$2)`, humanID, incidentID).Scan(&result)
	return result, err
}

func (s *AccessStore) HasSponsor(ctx context.Context, incidentID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_sponsors WHERE incident_id=$1)`, incidentID).Scan(&result)
	return result, err
}

func (s *AccessStore) IsController(ctx context.Context, humanID, incidentID, compartment string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.compartment_controllers WHERE human_id=$1 AND incident_id=$2 AND compartment=$3)`, humanID, incidentID, compartment).Scan(&result)
	return result, err
}

func (s *AccessStore) IsCustodian(ctx context.Context, humanID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.custodian_designations WHERE human_id=$1)`, humanID).Scan(&result)
	return result, err
}

// MajorIDs lists people eligible for the Major role.
func (s *AccessStore) MajorIDs(ctx context.Context) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT human_id FROM crab.role_eligibility WHERE role='Major' ORDER BY human_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out = append(out, id)
	}
	return out, rows.Err()
}

func (s *AccessStore) IsMajorEligible(ctx context.Context, humanID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.role_eligibility WHERE human_id=$1 AND role='Major')`, humanID).Scan(&result)
	return result, err
}

func (s *AccessStore) HasRoleEligibility(ctx context.Context, humanID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.role_eligibility WHERE human_id=$1)`, humanID).Scan(&result)
	return result, err
}

func (s *AccessStore) IncidentExists(ctx context.Context, incidentID string) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incidents WHERE incident_id=$1)`, incidentID).Scan(&result)
	return result, err
}

func (s *AccessStore) HasIncidentMembershipAt(ctx context.Context, humanID, incidentID string, now time.Time) (bool, error) {
	var result bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.incident_memberships WHERE human_id=$1 AND incident_id=$2 AND (effective_at IS NULL OR effective_at <= $3) AND (expires_at IS NULL OR expires_at > $3))`, humanID, incidentID, now).Scan(&result)
	return result, err
}

func (s *AccessStore) AdministrationView(ctx context.Context, now time.Time) (access.AdministrationView, error) {
	var view access.AdministrationView
	outputs, err := s.pool.Query(ctx, `SELECT human_id,action FROM crab.output_permissions ORDER BY human_id,action`)
	if err != nil {
		return view, err
	}
	for outputs.Next() {
		var v access.OutputPermissionView
		if err := outputs.Scan(&v.HumanID, &v.Action); err != nil {
			outputs.Close()
			return view, err
		}
		view.OutputPermissions = append(view.OutputPermissions, v)
	}
	outputs.Close()
	if err := outputs.Err(); err != nil {
		return view, err
	}
	rows, err := s.pool.Query(ctx, `SELECT human_id,incident_id FROM crab.incident_sponsors ORDER BY incident_id,human_id`)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		var v access.DesignationView
		if err := rows.Scan(&v.HumanID, &v.IncidentID); err != nil {
			rows.Close()
			return view, err
		}
		view.Sponsors = append(view.Sponsors, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return view, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT human_id,incident_id,compartment FROM crab.compartment_controllers ORDER BY incident_id,compartment,human_id`)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		var v access.DesignationView
		if err := rows.Scan(&v.HumanID, &v.IncidentID, &v.Compartment); err != nil {
			rows.Close()
			return view, err
		}
		view.Controllers = append(view.Controllers, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return view, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT human_id,incident_id,effective_at,expires_at,reason FROM crab.incident_memberships ORDER BY incident_id,human_id`)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		var v access.MembershipView
		var effective, expires sql.NullTime
		if err := rows.Scan(&v.HumanID, &v.IncidentID, &effective, &expires, &v.Reason); err != nil {
			rows.Close()
			return view, err
		}
		v.EffectiveAt, v.ExpiresAt = nullableTimes(effective, expires)
		v.Status = access.WindowStatus(v.EffectiveAt, v.ExpiresAt, now)
		view.IncidentMemberships = append(view.IncidentMemberships, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return view, err
	}
	rows.Close()

	rows, err = s.pool.Query(ctx, `SELECT human_id,incident_id,compartment,effective_at,expires_at,reason FROM crab.compartment_memberships ORDER BY incident_id,compartment,human_id`)
	if err != nil {
		return view, err
	}
	for rows.Next() {
		var v access.MembershipView
		var effective, expires sql.NullTime
		if err := rows.Scan(&v.HumanID, &v.IncidentID, &v.Compartment, &effective, &expires, &v.Reason); err != nil {
			rows.Close()
			return view, err
		}
		v.EffectiveAt, v.ExpiresAt = nullableTimes(effective, expires)
		v.Status = access.WindowStatus(v.EffectiveAt, v.ExpiresAt, now)
		v.Implicit = v.Compartment == "GENERAL"
		view.CompartmentMemberships = append(view.CompartmentMemberships, v)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return view, err
	}
	rows.Close()
	return view, nil
}

func nullableTimes(effective, expires sql.NullTime) (*time.Time, *time.Time) {
	var effectivePtr, expiresPtr *time.Time
	if effective.Valid {
		v := effective.Time
		effectivePtr = &v
	}
	if expires.Valid {
		v := expires.Time
		expiresPtr = &v
	}
	return effectivePtr, expiresPtr
}

func MarshalAccessData(v any) (json.RawMessage, error) {
	return json.Marshal(v)
}

// People lists everyone known to access administration: identity records,
// role eligibility and Custodian designations, with their currently active
// incident memberships at now.
func (s *AccessStore) People(ctx context.Context, now time.Time) ([]access.PersonRecord, error) {
	rows, err := s.pool.Query(ctx, `
		WITH ids AS (
			SELECT human_id FROM crab.identities
			UNION SELECT human_id FROM crab.role_eligibility
			UNION SELECT human_id FROM crab.custodian_designations)
		SELECT ids.human_id,
			i.human_id IS NOT NULL, COALESCE(i.display_name,''), COALESCE(i.organisation,''), COALESCE(i.external,false), COALESCE(i.status,''), i.account_expires_at,
			COALESCE(r.role,''), c.human_id IS NOT NULL,
			COALESCE((SELECT array_agg(m.incident_id ORDER BY m.incident_id) FROM crab.incident_memberships m
				WHERE m.human_id=ids.human_id AND (m.effective_at IS NULL OR m.effective_at <= $1) AND (m.expires_at IS NULL OR m.expires_at > $1)), '{}')
		FROM ids
		LEFT JOIN crab.identities i ON i.human_id=ids.human_id
		LEFT JOIN crab.role_eligibility r ON r.human_id=ids.human_id
		LEFT JOIN crab.custodian_designations c ON c.human_id=ids.human_id
		ORDER BY ids.human_id`, now)
	if err != nil {
		return nil, fmt.Errorf("list people: %w", err)
	}
	defer rows.Close()
	var out []access.PersonRecord
	for rows.Next() {
		var p access.PersonRecord
		var expires sql.NullTime
		if err := rows.Scan(&p.HumanID, &p.Identity.Found, &p.Identity.DisplayName, &p.Identity.Organisation, &p.Identity.External, &p.Identity.Status, &expires, &p.Role, &p.Custodian, &p.Incidents); err != nil {
			return nil, err
		}
		if expires.Valid {
			v := expires.Time.UTC()
			p.Identity.AccountExpiresAt = &v
		}
		p.Legacy = !p.Identity.Found && (p.Role != "" || p.Custodian)
		out = append(out, p)
	}
	return out, rows.Err()
}

// HasPendingIdentity reports whether an identity proposal for this person is
// waiting for confirmation.
func (s *AccessStore) HasPendingIdentity(ctx context.Context, humanID string) (bool, error) {
	var pending bool
	err := s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.access_proposals WHERE scope_id='' AND record_type=$1 AND convert_from(payload,'UTF8')::jsonb->>'human_id' = $2)`, access.RecordIdentity, humanID).Scan(&pending)
	return pending, err
}

// ExternalPeople returns the IDs of people recorded as external.
func (s *AccessStore) ExternalPeople(ctx context.Context) (map[string]bool, error) {
	rows, err := s.pool.Query(ctx, `SELECT human_id FROM crab.identities WHERE external`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]bool{}
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		out[id] = true
	}
	return out, rows.Err()
}
