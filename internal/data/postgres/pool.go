package postgres

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
)

const defaultMaxConns int32 = 8

// OpenPool creates the bounded PostgreSQL pool used by CRAB's data adapter.
// Connection strings are deployment configuration; business configuration does
// not belong here.
func OpenPool(ctx context.Context, databaseURL string) (*pgxpool.Pool, error) {
	if strings.TrimSpace(databaseURL) == "" {
		return nil, errors.New("database URL is required")
	}
	cfg, err := pgxpool.ParseConfig(databaseURL)
	if err != nil {
		return nil, fmt.Errorf("parse database URL: %w", err)
	}
	if cfg.MaxConns <= 0 || cfg.MaxConns > defaultMaxConns {
		cfg.MaxConns = defaultMaxConns
	}
	cfg.MinConns = 0
	cfg.MaxConnIdleTime = 5 * time.Minute
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		return nil, fmt.Errorf("open PostgreSQL pool: %w", err)
	}
	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := pool.Ping(pingCtx); err != nil {
		pool.Close()
		return nil, fmt.Errorf("ping PostgreSQL: %w", err)
	}
	return pool, nil
}

// ValidateRuntimeRole proves the ordinary application connection is not the
// schema/table owner, superuser, or BYPASSRLS role. Application authorisation
// remains primary; this prevents an accidental administrative runtime.
func ValidateRuntimeRole(ctx context.Context, pool *pgxpool.Pool) error {
	var role string
	var superuser, bypassRLS bool
	if err := pool.QueryRow(ctx, `
		SELECT rolname, rolsuper, rolbypassrls
		FROM pg_roles
		WHERE rolname = current_user`).Scan(&role, &superuser, &bypassRLS); err != nil {
		return fmt.Errorf("read runtime role: %w", err)
	}
	if superuser || bypassRLS {
		return fmt.Errorf("runtime role %q must not be superuser or BYPASSRLS", role)
	}

	var owner string
	if err := pool.QueryRow(ctx, `
		SELECT pg_get_userbyid(c.relowner)
		FROM pg_class c
		JOIN pg_namespace n ON n.oid = c.relnamespace
		WHERE n.nspname = 'crab' AND c.relname = 'ledger_entries'`).Scan(&owner); err != nil {
		return fmt.Errorf("read ledger owner: %w", err)
	}
	if owner == role {
		return fmt.Errorf("runtime role %q must not own crab.ledger_entries", role)
	}
	return nil
}

func ValidateSchema(ctx context.Context, pool *pgxpool.Pool) error {
	var version int
	if err := pool.QueryRow(ctx, `SELECT COALESCE(MAX(version), 0) FROM crab.schema_migrations`).Scan(&version); err != nil {
		return fmt.Errorf("CRAB schema unavailable: %w", err)
	}
	if version != currentMigrationVersion {
		return fmt.Errorf("CRAB schema version %d is unsupported; required %d", version, currentMigrationVersion)
	}
	return nil
}
