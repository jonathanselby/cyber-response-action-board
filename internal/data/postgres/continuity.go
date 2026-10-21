package postgres

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type ContinuityProjection struct{}

func (ContinuityProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	_, err := tx.Exec(ctx, `TRUNCATE crab.incident_resource_requests, crab.incident_handovers, crab.incident_severity_changes, crab.incident_scope_assessments, crab.continuity_proposals, crab.first_report_dispositions, crab.first_reports`)
	return err
}

func (ContinuityProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	r := entry.Record
	switch r.RecordType {
	case domain.TypeFirstReport:
		if r.IncidentMode != "system" || r.WriteSemantic != "record" {
			return errors.New("first report requires system record semantic")
		}
		v, err := domain.DecodePayload[domain.FirstReport](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.first_reports(report_id,payload,maker,recorded_at) VALUES($1,$2,$3,$4) ON CONFLICT (report_id) DO NOTHING`, r.RecordID, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeFirstReportDisposition:
		if r.IncidentMode != "system" || r.WriteSemantic != "record" {
			return errors.New("first report disposition requires system record semantic")
		}
		v, err := domain.DecodePayload[domain.FirstReportDisposition](r)
		if err != nil {
			return err
		}
		var exists bool
		if err := tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM crab.first_reports WHERE report_id=$1)`, v.ReportID).Scan(&exists); err != nil {
			return err
		}
		if !exists {
			return errors.New("first report disposition references unknown report")
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.first_report_dispositions(record_id,report_id,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5)`, r.RecordID, v.ReportID, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeScopeAssessment:
		if r.WriteSemantic != "record" {
			return errors.New("scope assessment requires record semantic")
		}
		v, err := domain.DecodePayload[domain.ScopeAssessment](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_scope_assessments(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6)`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeSeverityChange:
		return applySeverityChange(ctx, tx, r)
	case domain.TypeHandover:
		if r.WriteSemantic != "record" {
			return errors.New("handover requires record semantic")
		}
		v, err := domain.DecodePayload[domain.Handover](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_handovers(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6)`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	case domain.TypeResourceRequest:
		if r.WriteSemantic != "record" {
			return errors.New("resource request requires record semantic")
		}
		v, err := domain.DecodePayload[domain.ResourceRequest](r)
		if err != nil {
			return err
		}
		payload, err := json.Marshal(v)
		if err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.incident_resource_requests(record_id,incident_id,compartment,payload,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6) ON CONFLICT (record_id) DO UPDATE SET payload=EXCLUDED.payload,maker=EXCLUDED.maker,recorded_at=EXCLUDED.recorded_at`, r.RecordID, r.IncidentID, r.Compartment, payload, r.Maker, r.Timestamp)
		return err
	default:
		return nil
	}
}

func applySeverityChange(ctx context.Context, tx pgx.Tx, r ledger.Record) error {
	if r.WriteSemantic != "governed" {
		return errors.New("severity change requires governed semantic")
	}
	data, err := domain.DecodeGovernedData(r.Data)
	if err != nil {
		return err
	}
	if r.Status == "proposed" {
		if _, err := domain.DecodePayload[domain.SeverityChange](r); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.continuity_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9) ON CONFLICT (incident_id,record_type,record_id) DO UPDATE SET compartment=EXCLUDED.compartment,version=EXCLUDED.version,maker=EXCLUDED.maker,payload=EXCLUDED.payload,proposed_at=EXCLUDED.proposed_at,mode=EXCLUDED.mode`, r.IncidentID, r.RecordID, r.RecordType, r.Compartment, data.Version, r.Maker, []byte(data.Payload), r.Timestamp, r.IncidentMode)
		return err
	}
	if r.Status != "confirmed" && r.Status != "rejected" {
		return errors.New("invalid severity decision status")
	}
	var version, maker, compartment, mode string
	var payload []byte
	err = tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, r.IncidentID, r.RecordType, r.RecordID).Scan(&version, &maker, &compartment, &mode, &payload)
	if errors.Is(err, pgx.ErrNoRows) {
		return errors.New("severity decision has no matching pending proposal")
	}
	if err != nil {
		return err
	}
	if version != data.Version || maker != r.Maker || compartment != r.Compartment || mode != r.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("severity decision does not match exact pending proposal")
	}
	if r.Status == "confirmed" {
		v, err := domain.DecodePayload[domain.SeverityChange](r)
		if err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO crab.incident_severity_changes(record_id,incident_id,compartment,payload,maker,confirmer,confirmed_at) VALUES($1,$2,$3,$4,$5,$6,$7)`, r.RecordID, r.IncidentID, r.Compartment, data.Payload, r.Maker, r.Confirmer, r.Timestamp); err != nil {
			return err
		}
		if _, err = tx.Exec(ctx, `UPDATE crab.incidents SET severity=$2 WHERE incident_id=$1`, r.IncidentID, v.Severity); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM crab.continuity_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, r.IncidentID, r.RecordType, r.RecordID)
	return err
}

type ContinuityStore struct{ pool *pgxpool.Pool }

func NewContinuityStore(pool *pgxpool.Pool) (*ContinuityStore, error) {
	if pool == nil {
		return nil, errors.New("PostgreSQL pool is required")
	}
	return &ContinuityStore{pool: pool}, nil
}

func (s *ContinuityStore) FirstReports(ctx context.Context) ([]domain.FirstReportView, error) {
	rows, err := s.pool.Query(ctx, `SELECT f.report_id,f.payload,f.maker,f.recorded_at,d.payload,d.recorded_at FROM crab.first_reports f LEFT JOIN LATERAL (SELECT payload,recorded_at FROM crab.first_report_dispositions WHERE report_id=f.report_id ORDER BY recorded_at DESC,record_id DESC LIMIT 1) d ON true ORDER BY f.recorded_at DESC,f.report_id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.FirstReportView
	for rows.Next() {
		var v domain.FirstReportView
		var payload []byte
		var dispositionPayload []byte
		var dispositionAt *time.Time
		if err := rows.Scan(&v.ReportID, &payload, &v.Maker, &v.RecordedAt, &dispositionPayload, &dispositionAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(payload, &v.Report); err != nil {
			return nil, err
		}
		if len(dispositionPayload) > 0 {
			var d domain.FirstReportDisposition
			if err := json.Unmarshal(dispositionPayload, &d); err != nil {
				return nil, err
			}
			v.Disposition = &d
			v.DispositionAt = dispositionAt
		}
	out = append(out, v)
	}
	return out, rows.Err()
}

func (s *ContinuityStore) ScopeAssessments(ctx context.Context, incidentID, compartment string) ([]domain.ScopeAssessmentView, error) {
	rows, err := s.pool.Query(ctx, `SELECT record_id,compartment,payload,maker,recorded_at FROM crab.incident_scope_assessments WHERE incident_id=$1 AND compartment=$2 ORDER BY recorded_at DESC,record_id`, incidentID, compartment)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []domain.ScopeAssessmentView
	for rows.Next() {
		var v domain.ScopeAssessmentView
		var p []byte
		if err := rows.Scan(&v.RecordID, &I.Compartment, &p, &v.Maker, &v.RecordedAt); err != nil {
			return nil, err
		}
		if err := json.Unmarshal(p, &v.Assessment); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *ContinuityStore) SeverityChanges(ctx context.Context, incidentID, compartment string) ([]domain.SeverityChangeView, error"’° —&÷w2ÂW'"£Ò2çööÂåVW'’†7G‚Â4TÄT5B&V6÷&Eö–BÆ6ö×'FÖVçBÇ–ÆöBÆÖ¶W"Æ6öæf—&ÖW"Æ6öæf—&ÖVEöBe$ôÒ7&"æ–æ6–FVçE÷6WfW&—G•ö6†ævW2t„U$R–æ6–FVçEö–CÒCäB6ö×'FÖVçCÒC"õ$DU"%’6öæf—&ÖVEöBDU42Ç&V6÷&Eö–FÂ–æ6–FVçD”BÂ6ö×'FÖVçB ––bW'"Òæ–Â° —&WGW&âæ–ÂÂW'  —Ð –FVfW"&÷w2ä6Æ÷6R‚ —f"÷WBµÖFöÖ–âå6WfW&—G”6†ævUf–Wp –f÷"&÷w2äæW‡B‚’° —f"`¦FöÖ–âå6WfW&—G”6†ævUf–Wp —f" mu‰åÑ”($%¥˜•ÉÈ€èôÉ½ÝÌ¹M…¸ ™Ø¹I•½É‘%°€™Ø¹½µÁ…ÉÑµ•¹Ð°€™À°€™Ø¹5…­•È°€™Ø¹½¹™¥Éµ•È°€™Ø¹½¹™¥Éµ•‘Ð¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%¥˜•ÉÈ€èô©Í½¸¹U¹µ…ÉÍ¡…°¡À°€™Ø¹¡…¹”¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%½ÕÐ€ô…ÁÁ•¹¡½ÕÐ°Ø¤(%ô(%É•ÑÕÉ¸½ÕÐ°É½ÝÌ¹ÉÈ ¤)ô()™Õ¹Œ€¡Ì€©½¹Ñ¥¹Õ¥ÑåMÑ½É”¤!…¹‘½Ù•ÉÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¥¹¥‘•¹Ñ%°½µÁ…ÉÑµ•¹ÐÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹!…¹‘½Ù•ÉY¥•Ü°•ÉÉ½È¤ì(%É½ÝÌ°•ÉÈ€èôÌ¹Á½½°¹EÕ•Éä¡Ñà°M1PÉ•½É‘}¥±½µÁ…ÉÑµ•¹Ð±Á…å±½…±µ…­•È±É•½É‘•‘}…ÐI=4É…ˆ¹¥¹¥‘•¹Ñ}¡…¹‘½Ù•ÉÌ]!I¥¹¥‘•¹Ñ}¥ôÄ9½µÁ…ÉÑµ•¹ÐôÈ=IH	dÉ•½É‘•‘}…ÐM±É•½É‘}¥‘€°¥¹¥‘•¹Ñ%°½µÁ…ÉÑµ•¹Ð¤(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸¹¥°°•ÉÈ(%ô(%‘•™•ÈÉ½ÝÌ¹±½Í” ¤(%Ù…È½ÕÐmu‘½µ…¥¸¹!…¹‘½Ù•ÉY¥•Ü(%™½ÈÉ½ÝÌ¹9•áÐ ¤ì($%Ù…ÈØ‘½µ…¥¸¹!…¹‘½Ù•ÉY¥•Ü($%Ù…ÈÀmu‰åÑ”($%¥˜•ÉÈ€èôÉ½ÝÌ¹M…¸ ™Ø¹I•½É‘%°€™Ø¹½µÁ…ÉÑµ•¹Ð°€™À°€™Ø¹5…­•È°€™Ø¹I•½É‘•‘Ð¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%¥˜•ÉÈ€èô©Í½¸¹U¹µ…ÉÍ¡…°¡À°€™Ø¹!…¹‘½Ù•È¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%½ÕÐ€ô…ÁÁ•¹¡½ÕÐ°Ø¤(%ô(%É•ÑÕÉ¸½ÕÐ°É½ÝÌ¹ÉÈ ¤)ô()™Õ¹Œ€¡Ì€©½¹Ñ¥¹Õ¥ÑåMÑ½É”¤I•Í½ÕÉ•I•ÅÕ•ÍÑÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¥¹¥‘•¹Ñ%°½µÁ…ÉÑµ•¹ÐÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹I•Í½ÕÉ•I•ÅÕ•ÍÑY¥•Ü°•ÉÉ½È¤ì(%É½ÝÌ°•ÉÈ€èôÌ¹Á½½°¹EÕ•Éä¡Ñà°M1PÉ•½É‘}¥±½µÁ…ÉÑµ•¹Ð±Á…å±½…±µ…­•È±É•½É‘•‘}…ÐI=4É…ˆ¹¥¹¥‘•¹Ñ}É•Í½ÕÉ•}É•ÅÕ•ÍÑÌ]!I¥¹¥‘•¹Ñ}¥ôÄ9½µÁ…ÉÑµ•¹ÐôÈ=IH	dÉ•½É‘•‘}…ÐM±É•½É‘}¥‘€°¥¹¥‘•¹Ñ%°½µÁ…ÉÑµ•¹Ð¤(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸¹¥°°•ÉÈ(%ô(%‘•™•ÈÉ½ÝÌ¹±½Í” ¤(%Ù…È½ÕÐmu‘½µ…¥¸¹I•Í½ÕÉ•I•ÅÕ•ÍÑY¥•Ü(%™½ÈÉ½ÝÌ¹9•áÐ ¤ì($%Ù…ÈØ‘½µ…¥¸¹I•Í½ÕÉ•I•ÅÕ•ÍÑY¥•Ü($%Ù…ÈÀmu‰åÑ”($%¥˜•ÉÈ€èôÉ½ÝÌ¹M…¸ ™Ø¹I•½É‘%°€™Ø¹½µÁ…ÉÑµ•¹Ð°€™À°€™Ø¹5…­•È°€™Ø¹I•½É‘•‘Ð¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%¥˜•ÉÈ€èô©Í½¸¹U¹µ…ÉÍ¡…°¡À°€™Ø¹I•ÅÕ•ÍÐ¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸¹¥°°•ÉÈ($%ô($%½ÕÐ€ô…ÁÁ•¹¡½ÕÐ°Ø¤(%ô(%É•ÑÕÉ¸½ÕÐ°É½ÝÌ¹ÉÈ ¤)ô()ÑåÁ”½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”ÍÑÉÕÐì(%-•É¹•°€©-•É¹•°(%M¥¹•ÈM¥¹•È(%MÑ½É”€€©½¹Ñ¥¹Õ¥ÑåMÑ½É”(%	±½‰Ì€€©	±½‰MÑ½É”)ô()™Õ¹Œ9•Ý½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¡­•É¹•°€©-•É¹•°°Í¥¹•ÈM¥¹•È°ÍÑ½É”€©½¹Ñ¥¹Õ¥ÑåMÑ½É”°‰±½‰Ì€©	±½‰MÑ½É”¤€ ©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”°•ÉÉ½È¤ì(%¥˜­•É¹•°€ôô¹¥°ñðÍÑ½É”€ôô¹¥°ñð‰±½‰Ì€ôô¹¥°ñðÍ¥¹•È¹-•å%€ôô€ˆˆì($%É•ÑÕÉ¸¹¥°°•ÉÉ½ÉÌ¹9•Ü ‰½¹Ñ¥¹Õ¥ÑäÉÕ¹Ñ¥µ”É•ÅÕ¥É•Ì­•É¹•°°Í¥¹•È°ÍÑ½É”…¹‰±½ˆÍÑ½É”ˆ¤(%ô(%É•ÑÕÉ¸€™½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ•í-•É¹•°è­•É¹•°°M¥¹•ÈèÍ¥¹•È°MÑ½É”èÍÑ½É”°	±½‰Ìè‰±½‰Íô°¹¥°)ô()™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤AÕÑ¥ÉÍÑI•Á½ÉÑ	±½ˆ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°™¥±•¹…µ”°µ•‘¥…QåÁ”ÍÑÉ¥¹œ°ÍÉŒ¥¼¹I•…‘•È¤€¡‘½µ…¥¸¹	±½‰I••¥ÁÐ°•ÉÉ½È¤ì(%¥˜™¥±•¹…µ”€ôô€ˆˆñðÍÉŒ€ôô¹¥°ì($%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÉ½ÉÌ¹9•Ü ‰‰±½ˆ™¥±•¹…µ”…¹É•…‘•È…É”É•ÅÕ¥É•ˆ¤(%ô(%‰±½‰%°•ÉÈ€èô½Á…ÅÕ•	±½‰% ¤(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ(%ô(%Ñà°•ÉÈ€èôÈ¹	±½‰Ì¹Á½½°¹	•¥¸¡Ñà¤(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ(%ô(%‘•™•ÈÑà¹I½±±‰…¬¡Ñà¤(% €èôÍ¡„ÈÔØ¹9•Ü ¤(%‰Õ˜€èôµ…­”¡mu‰åÑ”°•Ù¥‘•¹•¡Õ¹­	åÑ•Ì¤(%Í…µÁ±”€èôµ…­”¡mu‰åÑ”°€À°€ÔÄÈ¤(%Ù…ÈÑ½Ñ…°¥¹ÐØÐ(%¡Õ¹­9¼€èô€À(%™½Èì($%¥˜•ÉÈ€èôÑà¹ÉÈ ¤ì•ÉÈ€„ô¹¥°ì($$%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ($%ô($%¸°É•…‘ÉÈ€èôÍÉŒ¹I•…¡‰Õ˜¤($%¥˜¸€ø€Àì($$%¥˜Ñ½Ñ…°­¥¹ÐØÐ¡¸¤€øÈ¹	±½‰Ì¹µ…á	åÑ•Ìì($$$%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°ÉÉ	±½‰Q½½1…É”($$%ô($$%Á¥•”€èô‰Õ™lé¹t($$%¥˜±•¸¡Í…µÁ±”¤€ð€ÔÄÈì($$$%¹••€èô€ÔÄÈ€´±•¸¡Í…µÁ±”¤($$$%¥˜¹••€ø¸ì($$$$%¹••€ô¸($$$%ô($$$%Í…µÁ±”€ô…ÁÁ•¹¡Í…µÁ±”°Á¥••lé¹••‘t¸¸¸¤($$%ô($$%|°|€ô ¹]É¥Ñ”¡Á¥•”¤($$%¥˜|°•ÉÈ€èôÑà¹á•Œ¡Ñà°%9MIP%9Q<É…ˆ¹‰±½‰}¡Õ¹­Ì¡‰±½‰}¥±¡Õ¹­}¹¼±‰åÑ•Ì¤Y1UL Ä°È°Ì¥€°‰±½‰%°¡Õ¹­9¼°Á¥•”¤ì•ÉÈ€„ô¹¥°ì($$$%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ($$%ô($$%Ñ½Ñ…°€¬ô¥¹ÐØÐ¡¸¤($$%¡Õ¹­9¼¬¬($%ô($%¥˜É•…‘ÉÈ€„ô¹¥°ì($$%¥˜•ÉÉ½ÉÌ¹%Ì¡É•…‘ÉÈ°¥¼¹=¤ì($$$%‰É•…¬($$%ô($$%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°É•…‘ÉÈ($%ô(%ô(%‘¥•ÍÐ€èô¡•à¹¹½‘•Q½MÑÉ¥¹œ¡ ¹MÕ´¡¹¥°¤¤(%‘•Ñ•Ñ•€èô¡ÑÑÀ¹•Ñ•Ñ½¹Ñ•¹ÑQåÁ”¡Í…µÁ±”¤(%¹½Ü€èôÑ¥µ”¹9½Ü ¤¹UQ ¤(%¥˜|°•ÉÈ€èôÑà¹á•Œ¡Ñà°%9MIP%9Q<É…ˆ¹‰±½‰Ì¡‰±½‰}¥±¥¹¥‘•¹Ñ}¥±µ½‘”±½µÁ…ÉÑµ•¹Ð±Í¡„ÈÔØ±Í¥é•}‰åÑ•Ì±½É¥¥¹…±}™¥±•¹…µ”±‘•±…É•‘}µ•‘¥…}ÑåÁ”±‘•Ñ•Ñ•‘}µ•‘¥…}ÑåÁ”±É•…Ñ•‘}…Ð¤Y1UL Ä°œœ°ÍåÍÑ•´œ°MeMQ4œ°È°Ì°Ð°Ô°Ø°Ü¥€°‰±½‰%°‘¥•ÍÐ°Ñ½Ñ…°°™¥±•¹…µ”°µ•‘¥…QåÁ”°‘•Ñ•Ñ•°¹½Ü¤ì•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ(%ô(%¥˜•ÉÈ€èôÑà¹½µµ¥Ð¡Ñà¤ì•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑíô°•ÉÈ(%ô(%É•ÑÕÉ¸‘½µ…¥¸¹	±½‰I••¥ÁÑí	±½‰%è‰±½‰%°M!ÈÔØè‘¥•ÍÐ°M¥é•	åÑ•ÌèÑ½Ñ…°°=É¥¥¹…±¥±•¹…µ”è™¥±•¹…µ”°•±…É•‘5•‘¥…QåÁ”èµ•‘¥…QåÁ”°•Ñ•Ñ•‘5•‘¥…QåÁ”è‘•Ñ•Ñ•‘ô°¹¥°)ô()™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤ÁÁ•¹‘½¹Ñ¥¹Õ¥Ñä¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°É•½É±•‘•È¹I•½É¤•ÉÉ½Èì(%|°•ÉÈ€èôÈ¹-•É¹•°¹ÁÁ•¹¡Ñà°É•½É°È¹M¥¹•È°½¹Ñ¥¹Õ¥ÑåAÉ½©•Ñ¥½¹íô¤(%É•ÑÕÉ¸•ÉÈ)ô()™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤A•¹‘¥¹½¹Ñ¥¹Õ¥Ñä¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¥¹¥‘•¹Ñ%°É•½É‘QåÁ”°É•½É‘%ÍÑÉ¥¹œ¤€¡±•‘•È¹I•½É°‰½½°°•ÉÉ½È¤ì(%Ù…ÈÙ•ÉÍ¥½¸°µ…­•È°½µÁ…ÉÑµ•¹Ð°µ½‘”ÍÑÉ¥¹œ(%Ù…ÈÁ…å±½…mu‰åÑ”(%Ù…ÈÁÉ½Á½Í•‘ÐÑ¥µ”¹Q¥µ”(%•ÉÈ€èôÈ¹MÑ½É”¹Á½½°¹EÕ•ÉåI½Ü¡Ñà°M1PÙ•ÉÍ¥½¸±µ…­•È±½µÁ…ÉÑµ•¹Ð±µ½‘”±Á…å±½…±ÁÉ½Á½Í•‘}…ÐI=4É…ˆ¹½¹Ñ¥¹Õ¥Ñå}ÁÉ½Á½Í…±Ì]!I¥¹¥‘•¹Ñ}¥ôÄ9É•½É‘}ÑåÁ”ôÈ9É•½É‘}¥ôÍ€°¥¹¥‘•¹Ñ%°É•½É‘QåÁ”°É•½É‘%¤¹M…¸ ™Ù•ÉÍ¥½¸°€™µ…­•È°€™½µÁ…ÉÑµ•¹Ð°€™µ½‘”°€™Á…å±½…°€™ÁÉ½Á½Í•‘Ð¤(%¥˜•ÉÉ½ÉÌ¹%Ì¡•ÉÈ°Áà¹ÉÉ9½I½ÝÌ¤ì($%É•ÑÕÉ¸±•‘•È¹I•½É‘íô°™…±Í”°¹¥°(%ô(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸±•‘•È¹I•½É‘íô°™…±Í”°•ÉÈ(%ô(%ÝÉ…ÁÁ•°•ÉÈ€èô©Í½¸¹5…ÉÍ¡…°¡‘½µ…¥¸¹½Ù•É¹•‘…Ñ…íY•ÉÍ¥½¸èÙ•ÉÍ¥½¸°A…å±½…è©Í½¸¹I…Ý5•ÍÍ…”¡Á…å±½…¥ô¤(%¥˜•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸±•‘•È¹I•½É‘íô°™…±Í”°•ÉÈ(%ô(%É•½É€èô±•‘•È¹I•½É‘íM¡•µ…Y•ÉÍ¥½¸è€Ä°Q¥µ•ÍÑ…µÀèÁÉ½Á½Í•‘Ð¹UQ ¤°Q¥µ•MÑ…ÑÕÌè€‰ÑÉÕÍÑ•ˆ°%¹¥‘•¹Ñ%è¥¹¥‘•¹Ñ%°%¹¥‘•¹Ñ5½‘”èµ½‘”°]É¥Ñ•M•µ…¹Ñ¥Œè€‰½Ù•É¹•ˆ°I•½É‘QåÁ”èÉ•½É‘QåÁ”°I•½É‘%èÉ•½É‘%°MÑ…ÑÕÌè€‰ÁÉ½Á½Í•ˆ°5…­•Èèµ…­•È°½µÁ…ÉÑµ•¹Ðè½µÁ…ÉÑµ•¹Ð°…Ñ„èÝÉ…ÁÁ•°5…­•ÉM¥¹…ÑÕÉ”è±•‘•È¹M¥¹…ÑÕÉ•í5¥ÍÍ¥¹I•…Í½¸è€‰µ…­•È…ÕÑ¡•¹Ñ¥…Ñ•½¸¥µµÕÑ…‰±”ÁÉ½Á½Í…°•¹ÑÉä‰õô(%¥˜|°•ÉÈ€èô‘½µ…¥¸¹AÉ½Á½Í…±Y•ÉÍ¥½¸¡É•½É¤ì•ÉÈ€„ô¹¥°ì($%É•ÑÕÉ¸±•‘•È¹I•½É‘íô°™…±Í”°•ÉÈ(%ô(%É•ÑÕÉ¸É•½É°ÑÉÕ”°¹¥°)ô()™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤¥ÉÍÑI•Á½ÉÑÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ¤€¡mu‘½µ…¥¸¹¥ÉÍÑI•Á½ÉÑY¥•Ü°•ÉÉ½È¤ì(%É•ÑÕÉ¸È¹MÑ½É”¹¥ÉÍÑI•Á½ÉÑÌ¡Ñà¤)ô)™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤M½Á•ÍÍ•ÍÍµ•¹ÑÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¤°ŒÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹M½Á•ÍÍ•ÍÍµ•¹ÑY¥•Ü°•ÉÉ½È¤ì(%É•ÑÕÉ¸È¹MÑ½É”¹M½Á•ÍÍ•ÍÍµ•¹ÑÌ¡Ñà°¤°Œ¤)ô)™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤M•Ù•É¥Ñå¡…¹•Ì¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¤°ŒÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹M•Ù•É¥Ñå¡…¹•Y¥•Ü°•ÉÉ½È¤ì(%É•ÑÕÉ¸È¹MÑ½É”¹M•Ù•É¥Ñå¡…¹•Ì¡Ñà°¤°Œ¤)ô)™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤!…¹‘½Ù•ÉÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¤°ŒÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹!…¹‘½Ù•ÉY¥•Ü°•ÉÉ½È¤ì(%É•ÑÕÉ¸È¹MÑ½É”¹!…¹‘½Ù•ÉÌ¡Ñà°¤°Œ¤)ô)™Õ¹Œ€¡È€©½¹Ñ¥¹Õ¥ÑåIÕ¹Ñ¥µ”¤I•Í½ÕÉ•I•ÅÕ•ÍÑÌ¡Ñà½¹Ñ•áÐ¹½¹Ñ•áÐ°¤°ŒÍÑÉ¥¹œ¤€¡mu‘½µ…¥¸¹I•Í½ÕÉ•I•ÅÕ•ÍÑY¥•Ü°•ÉÉ½È¤ì(%É•ÑÕÉ¸È¹MÑ½É”¹I•Í½ÕÉ•I•ÅÕ•ÍÑÌ¡Ñà°¤°Œ¤)ô()™Õ¹Œ½¹Ñ¥¹Õ¥ÑåMÕµµ…Éä¡É•½É±•‘•È¹I•½É¤ÍÑÉ¥¹œì(%ÍÝ¥Ñ É•½É¹I•½É‘QåÁ”ì(%…Í”‘½µ…¥¸¹QåÁ•M½Á•ÍÍ•ÍÍµ•¹Ðè($%Ø°|€èô‘½µ…¥¸¹•½‘•A…å±½…‘m‘½µ…¥¸¹M½Á•ÍÍ•ÍÍµ•¹Ñt¡É•½É¤($%É•ÑÕÉ¸€‰M½Á”½¥µÁ…Ðè€ˆ€¬Ø¹%µÁ…Ð(%…Í”‘½µ…¥¸¹QåÁ•M•Ù•É¥Ñå¡…¹”è($%Ø°|€èô‘½µ…¥¸¹•½‘•A…å±½…‘m‘½µ…¥¸¹M•Ù•É¥Ñå¡…¹•t¡É•½É¤($%É•ÑÕÉ¸€‰M•Ù•É¥Ñäè€ˆ€¬Ø¹M•Ù•É¥Ñä(%…Í”‘½µ…¥¸¹QåÁ•!…¹‘½Ù•Èè($%É•ÑÕÉ¸€‰M¡¥™Ð¡…¹‘½Ù•Èˆ(%…Í”‘½µ…¥¸¹QåÁ•I•Í½ÕÉ•I•ÅÕ•ÍÐè($%Ø°|€èô‘½µ…¥¸¹•½‘•A…å±½…‘m‘½µ…¥¸¹I•Í½ÕÉ•I•ÅÕ•ÍÑt¡É•½É¤($%É•ÑÕÉ¸€‰I•Í½ÕÉ”É•ÅÕ•ÍÐè€ˆ€¬Ø¹I•ÅÕ•ÍÐ(%‘•™…Õ±Ðè($%É•ÑÕÉ¸™µÐ¹MÁÉ¥¹Ñ˜ ˆ•Ìˆ°É•½É¹I•½É‘QåÁ”¤(%ô)ô(