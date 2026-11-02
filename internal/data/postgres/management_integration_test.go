package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type countingTracer struct{ n atomic.Int64 }

func (c *countingTracer) TraceQueryStart(ctx context.Context, _ *pgx.Conn, _ pgx.TraceQueryStartData) context.Context {
	c.n.Add(1)
	return ctx
}
func (c *countingTracer) TraceQueryEnd(context.Context, *pgx.Conn, pgx.TraceQueryEndData) {}

// T-199 (amended by C7): the management summary uses a fixed number of
// queries, independent of how many incidents the viewer can see.
func TestManagementSummaryUsesAFixedNumberOfQueries(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	now := time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
	cfg := db.runtime.Config().Copy()
	tracer := &countingTracer{}
	cfg.ConnConfig.Tracer = tracer
	pool, err := pgxpool.NewWithConfig(ctx, cfg)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	store, err := NewManagementStore(pool)
	if err != nil {
		t.Fatal(err)
	}
	queriesFor := func(n int) int64 {
		t.Helper()
		var incidents, compartments []string
		for i := 0; i < n; i++ {
			id := fmt.Sprintf("INC-Q%03d", i)
			if _, err := db.admin.Exec(ctx, `INSERT INTO crab.incidents(incident_id,mode,title,summary,severity,declaration_record_id,declared_at) VALUES($1,'live','t','s','Severity level 2','D-'||$1,$2) ON CONFLICT DO NOTHING`, id, now); err != nil {
				t.Fatal(err)
			}
			incidents, compartments = append(incidents, id, id), append(compartments, "GENERAL", "LEGAL")
		}
		tracer.n.Store(0)
		summary, err := store.ManagementSummary(ctx, incidents, compartments, now)
		if err != nil {
			t.Fatal(err)
		}
		if summary.ActiveIncidents != n {
			t.Fatalf("%d incidents in scope, summary counts %d", n, summary.ActiveIncidents)
		}
		return tracer.n.Load()
	}
	one, many := queriesFor(1), queriesFor(40)
	if one != ManagementQueries || many != ManagementQueries {
		t.Fatalf("queries: %d for 1 incident, %d for 40; want %d for both", one, many, ManagementQueries)
	}
}

// Migration 021 backfills decision loops from the ledger on upgrade, giving
// the same rows the projection builds, and runs only once.
func TestDecisionLoopBackfillMatchesProjection(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	pub, priv, _ := ed25519.GenerateKey(rand.Reader)
	kernel, err := OpenKernel(ctx, db.runtime, ledger.Keyring{"server": base64.StdEncoding.EncodeToString(pub)})
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: priv}
	t0 := time.Date(2026, 11, 2, 1, 0, 0, 0, time.UTC)
	for i, loop := range []time.Duration{10 * time.Minute, time.Hour, 0} {
		scope := domain.Scope{IncidentID: "INC-U", Mode: "live", Compartment: "GENERAL"}
		proposal, err := domain.ProposeDecision(scope, fmt.Sprintf("DEC-%d", i), "alice", t0, domain.Decision{Decision: "d", Rationale: "r"})
		if err != nil {
			t.Fatal(err)
		}
		if _, err := kernel.Append(ctx, proposal, signer); err != nil {
			t.Fatal(err)
		}
		if loop == 0 {
			continue // left pending
		}
		version, _ := domain.ProposalVersion(proposal)
		decided, err := domain.Decide(proposal, version, "bob", "confirmed", t0.Add(loop))
		if err != nil {
			t.Fatal(err)
		}
		if _, err := kernel.Append(ctx, decided, signer); err != nil {
			t.Fatal(err)
		}
	}
	snapshot := func() string {
		var s string
		if err := db.admin.QueryRow(ctx, `SELECT COALESCE(string_agg(record_type||record_id||incident_id||mode||compartment||proposed_at::text||COALESCE(decided_at::text,'-')||COALESCE(outcome,'-'), '|' ORDER BY record_id), '') FROM crab.decision_loops`).Scan(&s); err != nil {
			t.Fatal(err)
		}
		return s
	}
	projected := snapshot()
	if projected == "" {
		t.Fatal("kernel did not project decision loops")
	}
	if _, err := db.admin.Exec(ctx, `TRUNCATE crab.decision_loops; DELETE FROM crab.schema_migrations WHERE version=21`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db.owner, testRuntimeRole); err != nil {
		t.Fatal(err)
	}
	if got := snapshot(); got != projected {
		t.Fatalf("backfill differs from projection:\n%s\n%s", got, projected)
	}
	// A later start does not run the backfill again.
	if _, err := db.admin.Exec(ctx, `DELETE FROM crab.decision_loops WHERE record_id='DEC-0'`); err != nil {
		t.Fatal(err)
	}
	if err := Migrate(ctx, db.owner, testRuntimeRole); err != nil {
		t.Fatal(err)
	}
	if snapshot() == projected {
		t.Fatal("backfill ran again on a later start")
	}
}
