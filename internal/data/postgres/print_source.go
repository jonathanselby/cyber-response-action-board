package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type PrintStore struct{ pool *pgxpool.Pool }

func NewPrintStore(pool *pgxpool.Pool) (*PrintStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &PrintStore{pool: pool}, nil
}

func (s *PrintStore) LedgerSource(ctx context.Context, generatedAt time.Time, configurationVersion string) (domain.PrintSourceMark, error) {
	if generatedAt.IsZero() || configurationVersion == "" {
		return domain.PrintSourceMark{}, errors.New("print source requires generation time and configuration version")
	}
	var seq int64
	var hash string
	var raw []byte
	err := s.pool.QueryRow(ctx, `SELECT seq,hash,entry_bytes FROM crab.ledger_entries ORDER BY seq DESC LIMIT 1`).Scan(&seq, &hash, &raw)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.PrintSourceMark{}, errors.New("cannot generate sourced print artefact from empty ledger")
	}
	if err != nil {
		return domain.PrintSourceMark{}, err
	}
	var entry ledger.Entry
	if err := json.Unmarshal(raw, &entry); err != nil {
		return domain.PrintSourceMark{}, err
	}
	mark := domain.PrintSourceMark{LedgerSequence: uint64(seq), LedgerHash: hash, ConfigurationVersion: configurationVersion, GeneratedAt: generatedAt.UTC(), TimeStatus: entry.Record.TimeStatus}
	if err := domain.ValidatePrintSourceMark(mark); err != nil {
		return domain.PrintSourceMark{}, err
	}
	return mark, nil
}

func (s *PrintStore) NextCommunicationDeadline(ctx context.Context, incidentID, compartment string) (*time.Time, error) {
	if compartment == "" {
		compartment = "GENERAL"
	}
	var next *time.Time
	err := s.pool.QueryRow(ctx, `SELECT MIN(next_due_at) FROM crab.communication_cadences WHERE incident_id=$1 AND (compartment='GENERAL' OR compartment=$2)`, incidentID, compartment).Scan(&next)
	return next, err
}
