package postgres

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// NoticeProjection projects incident.notice records. The primary key on the
// notice ID makes a second append of the same notice fail, so an event can
// never notify the same person twice.
type NoticeProjection struct{}

func (NoticeProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.notices`)
	return err
}

func (NoticeProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if record.RecordType != domain.TypeNotice {
		return nil
	}
	if record.WriteSemantic != "record" || record.IncidentID == "" {
		return errors.New("notice records require record semantics and an incident")
	}
	value, err := domain.DecodePayload[domain.Notice](record)
	if err != nil {
		return err
	}
	if err := domain.ValidateNotice(value); err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.notices(notice_id,incident_id,compartment,kind,recipient_human_id,shared_channel,channel,status,detail,source_id,source_at,attempted_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`,
		value.NoticeID, record.IncidentID, record.Compartment, value.Kind, value.RecipientHumanID, value.SharedChannel, value.Channel, value.Status, value.Detail, value.SourceID, value.SourceAt, value.AttemptedAt)
	return err
}

// NoticeRuntime is the data side of the notice pass and of notification
// routes. It applies no policy.
type NoticeRuntime struct {
	Kernel *Kernel
	Signer Signer
	pool   *pgxpool.Pool
}

func NewNoticeRuntime(kernel *Kernel, signer Signer, pool *pgxpool.Pool) (*NoticeRuntime, error) {
	if kernel == nil || pool == nil || signer.KeyID == "" {
		return nil, errors.New("notice runtime requires kernel, signer and pool")
	}
	return &NoticeRuntime{Kernel: kernel, Signer: signer, pool: pool}, nil
}

func (r *NoticeRuntime) AppendNotice(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, NoticeProjection{})
	return err
}

// Cursor returns how far the notice pass has read; ok=false before the first pass.
func (r *NoticeRuntime) Cursor(ctx context.Context) (uint64, time.Time, bool, error) {
	var seq int64
	var started time.Time
	err := r.pool.QueryRow(ctx, `SELECT seq, started_at FROM crab.notice_cursor WHERE id=1`).Scan(&seq, &started)
	if errors.Is(err, pgx.ErrNoRows) {
		return 0, time.Time{}, false, nil
	}
	if err != nil {
		return 0, time.Time{}, false, err
	}
	return uint64(seq), started.UTC(), true, nil
}

func (r *NoticeRuntime) SetCursor(ctx context.Context, seq uint64, started time.Time) error {
	_, err := r.pool.Exec(ctx, `INSERT INTO crab.notice_cursor(id,seq,started_at) VALUES(1,$1,$2) ON CONFLICT(id) DO UPDATE SET seq=EXCLUDED.seq`, int64(seq), started.UTC())
	return err
}

func (r *NoticeRuntime) TailSeq(ctx context.Context) (uint64, error) {
	var seq int64
	err := r.pool.QueryRow(ctx, `SELECT COALESCE(max(seq),0) FROM crab.ledger_entries`).Scan(&seq)
	return uint64(seq), err
}

// EntriesSince returns committed ledger entries after seq, in order.
func (r *NoticeRuntime) EntriesSince(ctx context.Context, seq uint64, limit int) ([]ledger.Entry, error) {
	rows, err := r.pool.Query(ctx, `SELECT entry_bytes FROM crab.ledger_entries WHERE seq > $1 ORDER BY seq LIMIT $2`, int64(seq), limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ledger.Entry
	for rows.Next() {
		var raw []byte
		if err := rows.Scan(&raw); err != nil {
			return nil, err
		}
		var entry ledger.Entry
		if err := json.Unmarshal(raw, &entry); err != nil {
			return nil, fmt.Errorf("decode ledger entry: %w", err)
		}
		out = append(out, entry)
	}
	return out, rows.Err()
}

func (r *NoticeRuntime) NoticeExists(ctx context.Context, noticeID string) (bool, error) {
	var ok bool
	err := r.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.notices WHERE notice_id=$1)`, noticeID).Scan(&ok)
	return ok, err
}

// LastNotice returns the latest notice of a kind to a person in an incident.
func (r *NoticeRuntime) LastNotice(ctx context.Context, kind, humanID, incidentID string) (attemptedAt, sourceAt time.Time, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `SELECT attempted_at, source_at FROM crab.notices WHERE kind=$1 AND recipient_human_id=$2 AND incident_id=$3 ORDER BY attempted_at DESC, source_at DESC LIMIT 1`, kind, humanID, incidentID).Scan(&attemptedAt, &sourceAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, time.Time{}, false, nil
	}
	if err != nil {
		return time.Time{}, time.Time{}, false, err
	}
	return attemptedAt.UTC(), sourceAt.UTC(), true, nil
}

// Notices lists the notices of an incident, newest first.
func (r *NoticeRuntime) Notices(ctx context.Context, incidentID string, limit int) ([]domain.NoticeView, error) {
	rows, err := r.pool.Query(ctx, `SELECT incident_id,compartment,notice_id,kind,recipient_human_id,shared_channel,channel,status,detail,source_id,source_at,attempted_at FROM crab.notices WHERE incident_id=$1 ORDER BY attempted_at DESC, notice_id LIMIT $2`, incidentID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.NoticeView
	for rows.Next() {
		var v domain.NoticeView
		n := &v.Notice
		if err := rows.Scan(&v.IncidentID, &v.Compartment, &n.NoticeID, &n.Kind, &n.RecipientHumanID, &n.SharedChannel, &n.Channel, &n.Status, &n.Detail, &n.SourceID, &n.SourceAt, &n.AttemptedAt); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// IncidentsWithPending lists incidents that have any pending proposal.
func (r *NoticeRuntime) IncidentsWithPending(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT incident_id FROM crab.incident_proposals
		UNION SELECT incident_id FROM crab.communication_proposals
		UNION SELECT incident_id FROM crab.continuity_proposals
		UNION SELECT incident_id FROM crab.incident_recovery_proposals
		UNION SELECT incident_id FROM crab.incident_relationship_proposals
		UNION SELECT scope_id FROM crab.access_proposals WHERE scope_id <> ''
		UNION SELECT scope_id FROM crab.obligation_proposals WHERE scope_id <> ''
		ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	return collectStrings(rows)
}

// IncidentMembers lists an incident's current members.
func (r *NoticeRuntime) IncidentMembers(ctx context.Context, incidentID string) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT human_id FROM crab.incident_memberships WHERE incident_id=$1 ORDER BY human_id`, incidentID)
	if err != nil {
		return nil, err
	}
	return collectStrings(rows)
}

func (r *NoticeRuntime) IncidentMode(ctx context.Context, incidentID string) (string, bool, error) {
	var mode string
	err := r.pool.QueryRow(ctx, `SELECT mode FROM crab.incidents WHERE incident_id=$1`, incidentID).Scan(&mode)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	return mode, err == nil, err
}

// DueDeadlines lists obligation warnings and deadlines that fell due in
// (since, now] on clocks that are still active, with the clock owner.
func (r *NoticeRuntime) DueDeadlines(ctx context.Context, since, now time.Time, limit int) ([]domain.DueDeadline, error) {
	rows, err := r.pool.Query(ctx, `SELECT w.incident_id, i.mode, c.compartment, w.clock_id, w.wake_kind, w.wake_at, c.owner_text
		FROM crab.timed_wait_list w
		JOIN crab.obligation_clocks c ON c.incident_id=w.incident_id AND c.clock_id=w.clock_id
		JOIN crab.incidents i ON i.incident_id=w.incident_id
		WHERE w.wake_kind IN ('obligation_warning','obligation_deadline') AND c.status='active' AND w.wake_at > $1 AND w.wake_at <= $2
		ORDER BY w.wake_at, w.incident_id, w.clock_id LIMIT $3`, since, now, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.DueDeadline
	for rows.Next() {
		var d domain.DueDeadline
		if err := rows.Scan(&d.IncidentID, &d.Mode, &d.Compartment, &d.ClockID, &d.Kind, &d.WakeAt, &d.Owner); err != nil {
			return nil, err
		}
		d.WakeAt = d.WakeAt.UTC()
		out = append(out, d)
	}
	return out, rows.Err()
}

// ExpiringAccess lists temporary incident and compartment memberships that
// expire in (now, until], with the Sponsors or Controllers who can renew them.
func (r *NoticeRuntime) ExpiringAccess(ctx context.Context, now, until time.Time) ([]domain.ExpiringAccess, error) {
	rows, err := r.pool.Query(ctx, `SELECT m.human_id, m.incident_id, 'GENERAL', m.expires_at, i.mode,
			COALESCE((SELECT array_agg(s.human_id ORDER BY s.human_id) FROM crab.incident_sponsors s WHERE s.incident_id=m.incident_id), '{}')
		FROM crab.incident_memberships m JOIN crab.incidents i ON i.incident_id=m.incident_id
		WHERE m.expires_at > $1 AND m.expires_at <= $2
		UNION ALL
		SELECT m.human_id, m.incident_id, m.compartment, m.expires_at, i.mode,
			COALESCE((SELECT array_agg(c.human_id ORDER BY c.human_id) FROM crab.compartment_controllers c WHERE c.incident_id=m.incident_id AND c.compartment=m.compartment), '{}')
		FROM crab.compartment_memberships m JOIN crab.incidents i ON i.incident_id=m.incident_id
		WHERE m.compartment <> 'GENERAL' AND m.expires_at > $1 AND m.expires_at <= $2
		ORDER BY 2, 3, 1`, now, until)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ExpiringAccess
	for rows.Next() {
		var a domain.ExpiringAccess
		if err := rows.Scan(&a.HumanID, &a.IncidentID, &a.Compartment, &a.ExpiresAt, &a.Mode, &a.Renewers); err != nil {
			return nil, err
		}
		a.ExpiresAt = a.ExpiresAt.UTC()
		out = append(out, a)
	}
	return out, rows.Err()
}

// Route returns a person's notification route.
func (r *NoticeRuntime) Route(ctx context.Context, humanID string) (channel, address string, ok bool, err error) {
	err = r.pool.QueryRow(ctx, `SELECT channel, address FROM crab.notification_routes WHERE human_id=$1`, humanID).Scan(&channel, &address)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", "", false, nil
	}
	return channel, address, err == nil, err
}

// RoutedPeople lists who has a route (for readiness and display).
func (r *NoticeRuntime) RoutedPeople(ctx context.Context) (map[string]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT human_id, channel FROM crab.notification_routes`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]string{}
	for rows.Next() {
		var h, c string
		if err := rows.Scan(&h, &c); err != nil {
			return nil, err
		}
		out[h] = c
	}
	return out, rows.Err()
}

// DecisionMakersAndCustodians lists Majors and Custodians (readiness).
func (r *NoticeRuntime) DecisionMakersAndCustodians(ctx context.Context) ([]string, error) {
	rows, err := r.pool.Query(ctx, `SELECT human_id FROM crab.role_eligibility WHERE role='Major'
		UNION SELECT human_id FROM crab.custodian_designations ORDER BY 1`)
	if err != nil {
		return nil, err
	}
	return collectStrings(rows)
}

// SetRoute writes the audit record and the route in one transaction: the
// address goes to supporting state, the ledger gets only the masked hint.
// An empty address clears the route.
func (r *NoticeRuntime) SetRoute(ctx context.Context, audit ledger.Record, humanID, channel, address string, now time.Time) error {
	_, err := r.Kernel.Append(ctx, audit, r.Signer, routeWrite{humanID: humanID, channel: channel, address: address, now: now})
	return err
}

// routeWrite is applied inside the audit append's transaction only. It is not
// a rebuild projection: routes are supporting state the ledger cannot rebuild.
type routeWrite struct {
	humanID, channel, address string
	now                       time.Time
}

func (routeWrite) Reset(context.Context, pgx.Tx) error { return nil }

func (w routeWrite) Apply(ctx context.Context, tx pgx.Tx, _ ledger.Entry) error {
	if w.address == "" {
		_, err := tx.Exec(ctx, `DELETE FROM crab.notification_routes WHERE human_id=$1`, w.humanID)
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.notification_routes(human_id,channel,address,updated_at) VALUES($1,$2,$3,$4)
		ON CONFLICT(human_id) DO UPDATE SET channel=EXCLUDED.channel, address=EXCLUDED.address, updated_at=EXCLUDED.updated_at`, w.humanID, w.channel, w.address, w.now.UTC())
	return err
}

func collectStrings(rows pgx.Rows) ([]string, error) {
	defer rows.Close()
	var out []string
	for rows.Next() {
		var s string
		if err := rows.Scan(&s); err != nil {
			return nil, err
		}
		out = append(out, s)
	}
	return out, rows.Err()
}
