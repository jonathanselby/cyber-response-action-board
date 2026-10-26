package postgres

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
)

type CommunicationProjection struct{}

func (CommunicationProjection) Reset(ctx context.Context, tx pgx.Tx) error {
	if _, err := tx.Exec(ctx, `DELETE FROM crab.timed_wait_list WHERE wake_kind='communication_cadence'`); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `DELETE FROM crab.incident_timeline WHERE record_type IN ($1,$2)`, domain.TypeCommunicationIssue, domain.TypeCommunicationCadence); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `TRUNCATE crab.incoming_enquiries, crab.notification_outcomes, crab.communication_cadences, crab.communications, crab.communication_proposals, crab.communication_drafts`)
	return err
}

func (CommunicationProjection) Apply(ctx context.Context, tx pgx.Tx, entry ledger.Entry) error {
	record := entry.Record
	if !communicationType(record.RecordType) {
		return nil
	}
	if record.IncidentID == "" || (record.IncidentMode != "live" && record.IncidentMode != "exercise") {
		return errors.New("communication projection requires live/exercise incident scope")
	}
	if record.WriteSemantic == "record" {
		return applyCommunicationRecord(ctx, tx, record)
	}
	if record.WriteSemantic != "governed" {
		return errors.New("invalid communication write semantic")
	}
	if record.RecordType != domain.TypeCommunicationIssue && record.RecordType != domain.TypeCommunicationCadence {
		return errors.New("communication record type cannot use governed semantic")
	}
	data, err := domain.DecodeGovernedData(record.Data)
	if err != nil {
		return err
	}
	if record.Status == "proposed" {
		if err := validateCommunicationGovernedPayload(ctx, tx, record); err != nil {
			return err
		}
		_, err := tx.Exec(ctx, `INSERT INTO crab.communication_proposals(incident_id,record_id,record_type,compartment,version,maker,payload,proposed_at,mode) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, record.IncidentID, record.RecordID, record.RecordType, record.Compartment, data.Version, record.Maker, []byte(data.Payload), record.Timestamp, record.IncidentMode)
		return err
	}
	if record.Status != "confirmed" && record.Status != "rejected" {
		return errors.New("invalid communication governance status")
	}
	var version, maker, compartment, mode string
	var payload []byte
	if err := tx.QueryRow(ctx, `SELECT version,maker,compartment,mode,payload FROM crab.communication_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID).Scan(&version, &maker, &compartment, &mode, &payload); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("communication decision has no matching pending proposal")
		}
		return err
	}
	if version != data.Version || maker != record.Maker || compartment != record.Compartment || mode != record.IncidentMode || !bytes.Equal(payload, []byte(data.Payload)) {
		return errors.New("communication decision does not match exact pending proposal")
	}
	if record.Status == "confirmed" {
		if err := applyConfirmedCommunication(ctx, tx, record); err != nil {
			return err
		}
		summary := "Report cadence confirmed"
		if record.RecordType == domain.TypeCommunicationIssue {
			if value, err := domain.DecodePayload[domain.CommunicationIssue](record); err == nil {
				summary = "Communication approved: " + value.Subject
			}
		} else if value, err := domain.DecodePayload[domain.CommunicationCadence](record); err == nil {
			summary = "Report cadence confirmed: " + value.Name
		}
		if err := insertTimelineDecision(ctx, tx, entry, summary); err != nil {
			return err
		}
	}
	_, err = tx.Exec(ctx, `DELETE FROM crab.communication_proposals WHERE incident_id=$1 AND record_type=$2 AND record_id=$3`, record.IncidentID, record.RecordType, record.RecordID)
	return err
}

func applyCommunicationRecord(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeCommunicationDraft:
		value, err := domain.DecodePayload[domain.CommunicationDraft](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateCommunicationDraft(value); err != nil {
			return err
		}
		digest, err := domain.CommunicationDraftDigest(value)
		if err != nil {
			return err
		}
		audience, _ := json.Marshal(value.Audience)
		channels, _ := json.Marshal(value.Channels)
		sources, _ := json.Marshal(value.SourceRefs)
		related, _ := json.Marshal(value.RelatedRecordIDs)
		if _, err := tx.Exec(ctx, `INSERT INTO crab.communication_drafts(incident_id,record_id,mode,compartment,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,sha256,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15)`, record.IncidentID, record.RecordID, record.IncidentMode, record.Compartment, value.Kind, value.Subject, value.Body, audience, channels, sources, related, value.CadenceID, digest, record.Maker, record.Timestamp); err != nil {
			return err
		}
		if value.Kind == domain.CommunicationSituationReport && strings.TrimSpace(value.CadenceID) != "" {
			return advanceCommunicationCadence(ctx, tx, record.IncidentID, value.CadenceID, record.Timestamp)
		}
		return nil
	case domain.TypeNotificationOutcome:
		value, err := domain.DecodePayload[domain.NotificationOutcome](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateNotificationOutcome(value); err != nil {
			return err
		}
		_, err = tx.Exec(ctx, `INSERT INTO crab.notification_outcomes(incident_id,record_id,compartment,communication_id,channel,destination_id,attempted_at,status,detail,acknowledgement,duration_ms,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)`, record.IncidentID, record.RecordID, record.Compartment, value.CommunicationID, value.Channel, value.DestinationID, value.AttemptedAt, value.Status, value.Detail, value.Acknowledgement, value.DurationMS, record.Maker, record.Timestamp)
		return err
	case domain.TypeIncomingEnquiry:
		value, err := domain.DecodePayload[domain.IncomingEnquiry](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateIncomingEnquiry(value); err != nil {
			return err
		}
		related, _ := json.Marshal(value.RelatedRecordIDs)
		_, err = tx.Exec(ctx, `INSERT INTO crab.incoming_enquiries(incident_id,record_id,compartment,source,contact,subject,message,received_at,assigned_to_human_id,related_record_ids,maker,recorded_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)`, record.IncidentID, record.RecordID, record.Compartment, value.Source, value.Contact, value.Subject, value.Message, value.ReceivedAt, value.AssignedToHumanID, related, record.Maker, record.Timestamp)
		return err
	default:
		return errors.New("governed communication cannot bypass maker-checker through record semantic")
	}
}

func validateCommunicationGovernedPayload(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeCommunicationIssue:
		value, err := domain.DecodePayload[domain.CommunicationIssue](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateCommunicationIssue(value); err != nil {
			return err
		}
		var digest, compartment, mode string
		if err := tx.QueryRow(ctx, `SELECT sha256,compartment,mode FROM crab.communication_drafts WHERE incident_id=$1 AND record_id=$2`, record.IncidentID, value.DraftRecordID).Scan(&digest, &compartment, &mode); err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return errors.New("communication issue references unavailable draft")
			}
			return err
		}
		if digest != value.DraftSHA256 || compartment != record.Compartment || mode != record.IncidentMode {
			return errors.New("communication issue does not match immutable stored draft")
		}
		return nil
	case domain.TypeCommunicationCadence:
		value, err := domain.DecodePayload[domain.CommunicationCadence](record)
		if err != nil {
			return err
		}
		return domain.ValidateCommunicationCadence(value)
	default:
		return errors.New("unsupported governed communication type")
	}
}

func applyConfirmedCommunication(ctx context.Context, tx pgx.Tx, record ledger.Record) error {
	switch record.RecordType {
	case domain.TypeCommunicationIssue:
		value, err := domain.DecodePayload[domain.CommunicationIssue](record)
		if err != nil {
			return err
		}
		if err := validateCommunicationGovernedPayload(ctx, tx, record); err != nil {
			return err
		}
		audience, _ := json.Marshal(value.Audience)
		channels, _ := json.Marshal(value.Channels)
		sources, _ := json.Marshal(value.SourceRefs)
		related, _ := json.Marshal(value.RelatedRecordIDs)
		_, err = tx.Exec(ctx, `INSERT INTO crab.communications(incident_id,record_id,mode,compartment,draft_record_id,draft_sha256,kind,subject,body,audience,channels,source_refs,related_record_ids,cadence_id,maker,confirmer,issued_at) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)`, record.IncidentID, record.RecordID, record.IncidentMode, record.Compartment, value.DraftRecordID, value.DraftSHA256, value.Kind, value.Subject, value.Body, audience, channels, sources, related, value.CadenceID, record.Maker, record.Confirmer, record.Timestamp)
		return err
	case domain.TypeCommunicationCadence:
		value, err := domain.DecodePayload[domain.CommunicationCadence](record)
		if err != nil {
			return err
		}
		if err := domain.ValidateCommunicationCadence(value); err != nil {
			return err
		}
		audience, _ := json.Marshal(value.Audience)
		channels, _ := json.Marshal(value.Channels)
		simulated := record.IncidentMode == "exercise"
		nextDue := value.FirstDueAt.UTC()
		_, err = tx.Exec(ctx, `INSERT INTO crab.communication_cadences(incident_id,cadence_id,mode,compartment,name,interval_minutes,first_due_at,owner_human_id,audience,channels,maker,confirmer,confirmed_at,next_due_at,simulated) VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15) ON CONFLICT(incident_id,cadence_id) DO UPDATE SET mode=EXCLUDED.mode,compartment=EXCLUDED.compartment,name=EXCLUDED.name,interval_minutes=EXCLUDED.interval_minutes,first_due_at=EXCLUDED.first_due_at,owner_human_id=EXCLUDED.owner_human_id,audience=EXCLUDED.audience,channels=EXCLUDED.channels,maker=EXCLUDED.maker,confirmer=EXCLUDED.confirmer,confirmed_at=EXCLUDED.confirmed_at,last_generated_at=NULL,next_due_at=EXCLUDED.next_due_at,simulated=EXCLUDED.simulated`, record.IncidentID, record.RecordID, record.IncidentMode, record.Compartment, value.Name, value.IntervalMinutes, value.FirstDueAt, value.OwnerHumanID, audience, channels, record.Maker, record.Confirmer, record.Timestamp, nextDue, simulated)
		if err != nil {
			return err
		}
		return upsertCommunicationWake(ctx, tx, record.IncidentID, record.RecordID, nextDue, simulated)
	default:
		return errors.New("unsupported confirmed communication type")
	}
}

func advanceCommunicationCadence(ctx context.Context, tx pgx.Tx, incidentID, cadenceID string, generatedAt time.Time) error {
	var interval int
	var nextDue time.Time
	var simulated bool
	if err := tx.QueryRow(ctx, `SELECT interval_minutes,next_due_at,simulated FROM crab.communication_cadences WHERE incident_id=$1 AND cadence_id=$2`, incidentID, cadenceID).Scan(&interval, &nextDue, &simulated); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return errors.New("situation report references unavailable confirmed cadence")
		}
		return err
	}
	for !nextDue.After(generatedAt) {
		nextDue = nextDue.Add(time.Duration(interval) * time.Minute)
	}
	if _, err := tx.Exec(ctx, `UPDATE crab.communication_cadences SET last_generated_at=$3,next_due_at=$4 WHERE incident_id=$1 AND cadence_id=$2`, incidentID, cadenceID, generatedAt, nextDue); err != nil {
		return err
	}
	return upsertCommunicationWake(ctx, tx, incidentID, cadenceID, nextDue, simulated)
}

func upsertCommunicationWake(ctx context.Context, tx pgx.Tx, incidentID, cadenceID string, wakeAt time.Time, simulated bool) error {
	clockID := "communication:" + cadenceID
	if _, err := tx.Exec(ctx, `DELETE FROM crab.timed_wait_list WHERE incident_id=$1 AND clock_id=$2 AND wake_kind='communication_cadence'`, incidentID, clockID); err != nil {
		return err
	}
	_, err := tx.Exec(ctx, `INSERT INTO crab.timed_wait_list(incident_id,clock_id,wake_at,wake_kind,lifecycle,simulated) VALUES($1,$2,$3,'communication_cadence','firm',$4)`, incidentID, clockID, wakeAt, simulated)
	return err
}

func communicationType(recordType string) bool {
	switch recordType {
	case domain.TypeCommunicationDraft, domain.TypeCommunicationIssue, domain.TypeCommunicationCadence, domain.TypeNotificationOutcome, domain.TypeIncomingEnquiry:
		return true
	default:
		return false
	}
}
