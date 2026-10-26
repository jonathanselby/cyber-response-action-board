package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

// OrganisationProjection keeps pending and current organisation configuration
// (UAT2-BUILD-NOTES 4). It is rebuildable from the ledger.
type OrganisationProjection struct{}

func (OrganisationProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.organisation_proposals, crab.organisation_records`)
	return err
}

func (OrganisationProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !domain.IsOrganisationType(record.RecordType) {
		return nil
	}
	if record.WriteSemantic != "governed" || record.IncidentMode != "system" || record.Compartment != "SYSTEM" || record.IncidentID != "" {
		return errors.New("organisation configuration must be a governed SYSTEM record")
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	if record.Status == "proposed" {
		if err := domain.ValidateOrganisationPayload(record.RecordType, data.Payload); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO crab.organisation_proposals(record_id,record_type,version,maker,payload,proposed_at) VALUES($1,$2,$3,$4,$5,$6)`, record.RecordID, record.RecordType, data.Version, record.Maker, []byte(data.Payload), record.Timestamp)
		return err
	}
	var recordType, version, maker string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT record_type,version,maker,payload FROM crab.organisation_proposals WHERE record_id=$1`, record.RecordID).Scan(&recordType, &version, &maker, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("organisation decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	if recordType != record.RecordType || version != data.Version || maker != record.Maker || !jsonEqual(payload, data.Payload) {
		return errors.New("organisation decision does not match exact pending proposal")
	}
	if _, err := tx.Exec(ctx, `DELETE FROM crab.organisation_proposals WHERE record_id=$1`, record.RecordID); err != nil {
		return err
	}
	if record.Status != "confirmed" {
		return nil
	}
	key, err := domain.OrganisationRecordKey(record.RecordType, data.Payload)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `INSERT INTO crab.organisation_records(record_type,record_key,record_id,version,payload,maker,confirmer,confirmed_at,time_status,self_approval)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT(record_type,record_key) DO UPDATE SET record_id=EXCLUDED.record_id,version=EXCLUDED.version,payload=EXCLUDED.payload,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at,time_status=EXCLUDED.time_status,self_approval=EXCLUDED.self_approval`,
		record.RecordType, key, record.RecordID, data.Version, []byte(data.Payload), record.Maker, record.Confirmer, record.Timestamp, record.TimeStatus, record.SelfApproval)
	return err
}

func jsonEqual(a, b []byte) bool {
	var x, y any
	if json.Unmarshal(a, &x) != nil || json.Unmarshal(b, &y) != nil {
		return bytes.Equal(a, b)
	}
	xa, _ := json.Marshal(x)
	ya, _ := json.Marshal(y)
	return bytes.Equal(xa, ya)
}

// OrganisationRuntime appends and reads organisation configuration.
type OrganisationRuntime struct {
	Kernel *Kernel
	Signer Signer
}

func NewOrganisationRuntime(kernel *Kernel, signer Signer) (*OrganisationRuntime, error) {
	if kernel == nil || signer.KeyID == "" {
		return nil, errors.New("organisation runtime requires kernel and signer")
	}
	return &OrganisationRuntime{Kernel: kernel, Signer: signer}, nil
}

func (r *OrganisationRuntime) AppendOrganisation(ctx context.Context, record ledger.Record) error {
	_, err := r.Kernel.Append(ctx, record, r.Signer, OrganisationProjection{})
	return err
}

// PendingOrganisation rebuilds the stored proposal so a decision binds the
// exact version that was proposed.
func (r *OrganisationRuntime) PendingOrganisation(ctx context.Context, recordID string) (ledger.Record, bool, error) {
	var recordType, version, maker string
	var payload []byte
	var proposedAt time.Time
	err := r.Kernel.pool.QueryRow(ctx, `SELECT record_type,version,maker,payload,proposed_at FROM crab.organisation_proposals WHERE record_id=$1`, recordID).Scan(&recordType, &version, &maker, &payload, &proposedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return ledger.Record{}, false, nil
	}
	if err != nil {
		return ledger.Record{}, false, err
	}
	wrapped, err := json.Marshal(domain.GovernedData{Version: version, Payload: json.RawMessage(payload)})
	if err != nil {
		return ledger.Record{}, false, err
	}
	record := ledger.Record{SchemaVersion: 1, Timestamp: proposedAt.UTC(), TimeStatus: ledger.TimeTrusted, IncidentMode: "system", WriteSemantic: "governed", RecordType: recordType, RecordID: recordID, Status: "proposed", Maker: maker, Compartment: "SYSTEM", Data: wrapped, MakerSignature: ledger.Signature{MissingReason: "maker authenticated on immutable proposal entry"}}
	if _, err := domain.ProposalVersion(record); err != nil {
		return ledger.Record{}, false, err
	}
	return record, true, nil
}

// PendingOrganisationList lists every pending organisation proposal.
func (r *OrganisationRuntime) PendingOrganisationList(ctx context.Context) ([]domain.OrganisationView, error) {
	rows, err := r.Kernel.pool.Query(ctx, `SELECT record_id,record_type,version,payload,maker,proposed_at FROM crab.organisation_proposals ORDER BY proposed_at,record_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrganisationView
	for rows.Next() {
		var v domain.OrganisationView
		if err := rows.Scan(&v.RecordID, &v.RecordType, &v.Version, &v.Payload, &v.Maker, &v.At); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

// CurrentOrganisation returns the confirmed records of one type, newest first.
func (r *OrganisationRuntime) CurrentOrganisation(ctx context.Context, recordType string) ([]domain.OrganisationView, error) {
	rows, err := r.Kernel.pool.Query(ctx, `SELECT record_id,record_type,version,payload,maker,confirmer,confirmed_at,time_status,self_approval FROM crab.organisation_records WHERE record_type=$1 ORDER BY confirmed_at DESC,record_key`, recordType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.OrganisationView
	for rows.Next() {
		var v domain.OrganisationView
		if err := rows.Scan(&v.RecordID, &v.RecordType, &v.Version, &v.Payload, &v.Maker, &v.Confirmer, &v.At, &v.TimeStatus, &v.SelfApprove); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}
