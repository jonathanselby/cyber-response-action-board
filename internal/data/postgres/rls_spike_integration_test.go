package postgres

import (
	"context"
	"testing"
)

func TestRLSSpikePoolIsolationAndRuntimeBoundary(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()

	if _, err := db.owner.Exec(ctx, `
		CREATE TABLE crab.rls_spike_rows (
			incident_id text NOT NULL,
			compartment text NOT NULL,
			body text NOT NULL
		);
		INSERT INTO crab.rls_spike_rows(incident_id,compartment,body) VALUES
			('INC-A','GENERAL','a-general'),
			('INC-A','LEGAL','a-legal'),
			('INC-B','GENERAL','b-general'),
			('INC-B','LEGAL','b-legal');
		ALTER TABLE crab.rls_spike_rows ENABLE ROW LEVEL SECURITY;
		ALTER TABLE crab.rls_spike_rows FORCE ROW LEVEL SECURITY;
		CREATE POLICY rls_spike_scope ON crab.rls_spike_rows
			FOR ALL
			USING (
				incident_id = current_setting('crab.rls_incident', true)
				AND (
					compartment = 'GENERAL'
					OR compartment = current_setting('crab.rls_compartment', true)
				)
			)
			WITH CHECK (
				incident_id = current_setting('crab.rls_incident', true)
				AND (
					compartment = 'GENERAL'
					OR compartment = current_setting('crab.rls_compartment', true)
				)
			);
		GRANT SELECT, INSERT ON crab.rls_spike_rows TO `+testRuntimeRole); err != nil {
		t.Fatal(err)
	}

	var superuser, bypass bool
	if err := db.runtime.QueryRow(ctx, `SELECT rolsuper, rolbypassrls FROM pg_roles WHERE rolname=current_user`).Scan(&superuser, &bypass); err != nil {
		t.Fatal(err)
	}
	if superuser || bypass {
		t.Fatalf("runtime role escaped RLS boundary superuser=%v bypassrls=%v", superuser, bypass)
	}
	var owner string
	if err := db.runtime.QueryRow(ctx, `SELECT pg_get_userbyid(c.relowner) FROM pg_class c JOIN pg_namespace n ON n.oid=c.relnamespace WHERE n.nspname='crab' AND c.relname='rls_spike_rows'`).Scan(&owner); err != nil {
		t.Fatal(err)
	}
	if owner == testRuntimeRole {
		t.Fatal("runtime role owns RLS table")
	}

	conn, err := db.runtime.Acquire(ctx)
	if err != nil {
		t.Fatal(err)
	}
	defer conn.Release()

	var count int
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM crab.rls_spike_rows`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("missing scope leaked %d rows", count)
	}

	tx, err := conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('crab.rls_incident','INC-A',true), set_config('crab.rls_compartment','LEGAL',true)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM crab.rls_spike_rows`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("INC-A LEGAL scope saw %d rows want GENERAL+LEGAL", count)
	}
	if err := tx.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	if err := conn.QueryRow(ctx, `SELECT count(*) FROM crab.rls_spike_rows`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("transaction-local scope leaked after commit: %d rows", count)
	}

	tx, err = conn.Begin(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := tx.Exec(ctx, `SELECT set_config('crab.rls_incident','INC-B',true), set_config('crab.rls_compartment','GENERAL',true)`); err != nil {
		t.Fatal(err)
	}
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM crab.rls_spike_rows`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 1 {
		t.Fatalf("INC-B GENERAL scope saw %d rows", count)
	}
	if err := tx.Rollback(ctx); err != nil {
		t.Fatal(err)
	}
	if err := conn.QueryRow(ctx, `SELECT count(*) FROM crab.rls_spike_rows`).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 0 {
		t.Fatalf("transaction-local scope leaked after rollback: %d rows", count)
	}

	if _, err := db.runtime.Exec(ctx, `INSERT INTO crab.rls_spike_rows(incident_id,compartment,body) VALUES('INC-A','GENERAL','projection-write')`); err == nil {
		t.Fatal("unscoped runtime projection-style write unexpectedly passed RLS")
	}
}
