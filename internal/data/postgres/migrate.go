package postgres

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	currentMigrationVersion = 17
	migrationLockKey        = int64(0x435241424d494752)
)

//go:embed migrations/001_slice1.sql
var migration001 string

//go:embed migrations/002_security_support.sql
var migration002 string

//go:embed migrations/003_access_projection.sql
var migration003 string

//go:embed migrations/004_auth_credentials.sql
var migration004 string

//go:embed migrations/005_security_state.sql
var migration005 string

//go:embed migrations/006_incident_projection.sql
var migration006 string

//go:embed migrations/007_proposal_scope.sql
var migration007 string

//go:embed migrations/008_actions_checklists_timeline.sql
var migration008 string

//go:embed migrations/009_obligations.sql
var migration009 string

//go:embed migrations/010_access_governance.sql
var migration010 string

//go:embed migrations/011_evidence_blobs.sql
var migration011 string

//go:embed migrations/012_corpus_retrieval.sql
var migration012 string

//go:embed migrations/013_communications.sql
var migration013 string

//go:embed migrations/014_recovery_closure.sql
var migration014 string

//go:embed migrations/015_continuity.sql
var migration015 string

//go:embed migrations/016_organisation.sql
var migration016 string

//go:embed migrations/017_relationships.sql
var migration017 string

func Migrate(ctx context.Context, owner *pgxpool.Pool, runtimeRole string) error {
	runtimeRole = strings.TrimSpace(runtimeRole)
	if runtimeRole == "" {
		return errors.New("runtime role is required")
	}
	var exists bool
	if err := owner.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM pg_roles WHERE rolname = $1)`, runtimeRole).Scan(&exists); err != nil {
		return fmt.Errorf("check runtime role: %w", err)
	}
	if !exists {
		return fmt.Errorf("runtime role %q does not exist", runtimeRole)
	}
	tx, err := owner.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin migration: %w", err)
	}
	defer tx.Rollback(ctx)
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, migrationLockKey); err != nil {
		return fmt.Errorf("lock migration: %w", err)
	}
	migrations := []string{migration001, migration002, migration003, migration004, migration005, migration006, migration007, migration008, migration009, migration010, migration011, migration012, migration013, migration014, migration015, migration016, migration017}
	for version, migration := range migrations {
		if _, err := tx.Exec(ctx, migration); err != nil {
			return fmt.Errorf("migration %03d: %w", version+1, err)
		}
	}
	role := pgx.Identifier{runtimeRole}.Sanitize()
	grants := []string{
		`GRANT USAGE ON SCHEMA crab TO ` + role,
		`GRANT SELECT ON crab.schema_migrations TO ` + role,
		`GRANT SELECT, INSERT ON crab.ledger_entries TO ` + role,
		`GRANT SELECT, INSERT, UPDATE ON crab.local_credentials TO ` + role,
		`GRANT SELECT, INSERT, DELETE ON crab.sessions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.role_eligibility TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.custodian_designations TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_sponsors TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.compartment_controllers TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_memberships TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.compartment_memberships TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.access_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.output_permissions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.totp_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.passkey_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.break_glass_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.security_state TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incidents TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_facts TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_uncertainties TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_decisions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_actions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_operational_roles TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_timeline TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.obligation_definitions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.obligation_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.obligation_clocks TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.timed_wait_list TO ` + role,
		`GRANT SELECT, INSERT ON crab.blobs TO ` + role,
		`GRANT SELECT, INSERT ON crab.blob_chunks TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.evidence_register TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.evidence_handling TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.corpus_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.corpus_sources TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.corpus_passages TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.communication_drafts TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.communication_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.communications TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.communication_cadences TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.notification_outcomes TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incoming_enquiries TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_recovery_updates TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_recovery_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_recovery_state TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.first_reports TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.first_report_dispositions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.continuity_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_scope_assessments TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_severity_changes TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_handovers TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_resource_requests TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.organisation_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.organisation_records TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_relationship_proposals TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_relationships TO ` + role,
	}
	for _, statement := range grants {
		if _, err := tx.Exec(ctx, statement); err != nil {
			return fmt.Errorf("grant runtime privilege: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit migration: %w", err)
	}
	return nil
}
