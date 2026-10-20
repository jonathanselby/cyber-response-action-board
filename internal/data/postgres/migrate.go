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
	currentMigrationVersion = 5
	migrationLockKey        = int64(0x435241424d494752) // "CRABMIGR"
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
	for version, migration := range []string{migration001, migration002, migration003, migration004, migration005} {
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
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.incident_memberships TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.compartment_memberships TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.output_permissions TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.totp_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.passkey_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE ON crab.break_glass_credentials TO ` + role,
		`GRANT SELECT, INSERT, UPDATE, DELETE, TRUNCATE ON crab.security_state TO ` + role,
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
