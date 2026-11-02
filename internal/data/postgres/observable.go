package postgres

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// ObservableProjection keeps every version of every incident.observable.
type ObservableProjection struct{}

func (ObservableProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.observable_versions`)
	return err
}

func (ObservableProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeObservable {
		return nil
	}
	if record.WriteSemantic != "record" || record.Status != "recorded" {
		return errors.New("observables require immediate record semantics")
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") || record.Compartment == "" {
		return errors.New("observables require incident scope")
	}
	value, err := domain.DecodePayload[domain.Observable](record)
	if err != nil {
		return err
	}
	if err := domain.ValidateObservable(value); err != nil {
		return err
	}
	// A later version keeps the observable's incident, compartment, type,
	// value and first-seen time; only status, source and note change.
	var version int
	var incidentID, compartment, kind, normal string
	var firstSeen time.Time
	err = tx.QueryRow(ctx, `SELECT version,incident_id,compartment,type,value,first_seen FROM crab.observable_versions WHERE observable_id=$1 ORDER BY version DESC LIMIT 1`, value.ObservableID).
		Scan(&version, &incidentID, &compartment, &kind, &normal, &firstSeen)
	switch {
	case errors.Is(err, pgx.ErrNoRows):
		version = 0
	case err != nil:
		return err
	default:
		if incidentID != record.IncidentID || compartment != record.Compartment || kind != value.Type || normal != value.Value || !firstSeen.Equal(value.FirstSeen) {
			return errors.New("an observable version must keep its incident, compartment, type, value and first-seen time")
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.observable_versions(record_id,observable_id,version,incident_id,mode,compartment,type,value,status,source,first_seen,time_confidence,note,maker,recorded_at)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`,
		record.RecordID, value.ObservableID, version+1, record.IncidentID, record.IncidentMode, record.Compartment, value.Type, value.Value, value.Status, value.Source, value.FirstSeen, value.TimeConfidence, value.Note, record.Maker, record.Timestamp)
	return err
}

// ObservableStore reads observables.
type ObservableStore struct{ pool *pgxpool.Pool }

func NewObservableStore(pool *pgxpool.Pool) (*ObservableStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ObservableStore{pool: pool}, nil
}

const observableColumns = `record_id,observable_id,version,incident_id,mode,compartment,type,value,status,source,first_seen,time_confidence,note,maker,recorded_at`

func scanObservable(row pgx.Row) (domain.ObservableView, error) {
	var v domain.ObservableView
	err := row.Scan(&v.RecordID, &v.Observable.ObservableID, &v.Version, &v.IncidentID, &v.Mode, &v.Compartment, &v.Observable.Type, &v.Observable.Value, &v.Observable.Status, &v.Observable.Source, &v.Observable.FirstSeen, &v.Observable.TimeConfidence, &v.Observable.Note, &v.Maker, &v.RecordedAt)
	v.Observable.FirstSeen, v.RecordedAt = v.Observable.FirstSeen.UTC(), v.RecordedAt.UTC()
	return v, err
}

// Current returns the current version of one observable.
func (s *ObservableStore) Current(ctx context.Context, observableID string) (domain.ObservableView, bool, error) {
	v, err := scanObservable(s.pool.QueryRow(ctx, `SELECT `+observableColumns+` FROM crab.observable_versions WHERE observable_id=$1 ORDER BY version DESC LIMIT 1`, observableID))
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.ObservableView{}, false, nil
	}
	if err != nil {
		return domain.ObservableView{}, false, fmt.Errorf("read observable: %w", err)
	}
	return v, true, nil
}

// Register lists the current version of each observable in an incident's
// GENERAL compartment and, if given, one restricted compartment.
func (s *ObservableStore) Register(ctx context.Context, incidentID, compartment string) ([]domain.ObservableView, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+observableColumns+` FROM (
		SELECT DISTINCT ON (observable_id) * FROM crab.observable_versions
		WHERE incident_id=$1 AND (compartment='GENERAL' OR compartment=$2)
		ORDER BY observable_id, version DESC) o ORDER BY first_seen, observable_id`, incidentID, compartment)
	if err != nil {
		return nil, fmt.Errorf("list observables: %w", err)
	}
	defer rows.Close()
	var out []domain.ObservableView
	for rows.Next() {
		v, err := scanObservable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// History lists every version of one observable, oldest first.
func (s *ObservableStore) History(ctx context.Context, observableID string) ([]domain.ObservableView, error) {
	rows, err := s.pool.Query(ctx, `SELECT `+observableColumns+` FROM crab.observable_versions WHERE observable_id=$1 ORDER BY version`, observableID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ObservableView
	for rows.Next() {
		v, err := scanObservable(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// Sightings returns, for each (type, value) given, the other incidents and
// compartments holding the same normalised value. One query for the page.
func (s *ObservableStore) Sightings(ctx context.Context, incidentID string, kinds, values []string) (map[string][]domain.ObservableSighting, error) {
	out := map[string][]domain.ObservableSighting{}
	if len(kinds) == 0 {
		return out, nil
	}
	rows, err := s.pool.Query(ctx, `SELECT DISTINCT o.type, o.value, o.incident_id, o.compartment
		FROM crab.observable_versions o
		JOIN unnest($2::text[], $3::text[]) AS q(type, value) ON q.type=o.type AND q.value=o.value
		WHERE o.incident_id <> $1 ORDER BY o.incident_id, o.compartment`, incidentID, kinds, values)
	if err != nil {
		return nil, fmt.Errorf("read observable sightings: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var kind, value string
		var sight domain.ObservableSighting
		if err := rows.Scan(&kind, &value, &sight.IncidentID, &sight.Compartment); err != nil {
			return nil, err
		}
		key := kind + "\x00" + value
		out[key] = append(out[key], sight)
	}
	return out, rows.Err()
}

// ObservableRuntime appends observables through the kernel.
type ObservableRuntime struct {
	Kernel *Kernel
	Signer Signer
	*ObservableStore
}

func NewObservableRuntime(kernel *Kernel, signer Signer, store *ObservableStore) (*ObservableRuntime, error) {
	if kernel == nil || signer.KeyID == "" || store == nil {
		return nil, errors.New("observable runtime requires kernel, signer and store")
	}
	return &ObservableRuntime{Kernel: kernel, Signer: signer, ObservableStore: store}, nil
}

func (r *ObservableRuntime) AppendObservable(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, ObservableProjection{})
	return err
}
