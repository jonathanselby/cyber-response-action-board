package ceremony

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

const (
	RecordBootstrap              = "security.bootstrap"
	RecordCustodianRecovery      = "security.custodian_recovery"
	RecordBreakGlass             = "security.break_glass"
	RecordBreakGlassAlertAttempt = "security.break_glass_alert_attempt"
)

type BootstrapIdentity struct {
	HumanID             string
	Custodian           bool
	MajorEligible       bool
	LocalAuthConfigured bool
}

type BootstrapPlan struct {
	PreparedBy             string
	ConfirmedBy            string
	Identities             []BootstrapIdentity
	BreakGlassKeyholderIDs []string
}

func ValidateBootstrap(plan BootstrapPlan) error {
	if strings.TrimSpace(plan.PreparedBy) == "" || strings.TrimSpace(plan.ConfirmedBy) == "" || plan.PreparedBy == plan.ConfirmedBy {
		return errors.New("bootstrap requires two distinct named humans")
	}
	if len(plan.Identities) < 2 {
		return errors.New("bootstrap requires at least two named identities")
	}
	seen := map[string]BootstrapIdentity{}
	custodians, majors, local := 0, 0, 0
	for _, identity := range plan.Identities {
		identity.HumanID = strings.TrimSpace(identity.HumanID)
		if identity.HumanID == "" {
			return errors.New("bootstrap identity id is required")
		}
		if _, exists := seen[identity.HumanID]; exists {
			return errors.New("duplicate bootstrap identity")
		}
		seen[identity.HumanID] = identity
		if identity.Custodian {
			custodians++
		}
		if identity.MajorEligible {
			majors++
		}
		if identity.LocalAuthConfigured {
			local++
		}
	}
	preparer, preparerExists := seen[plan.PreparedBy]
	confirmer, confirmerExists := seen[plan.ConfirmedBy]
	if !preparerExists || !confirmerExists || !preparer.LocalAuthConfigured || !confirmer.LocalAuthConfigured {
		return errors.New("bootstrap preparer and confirmer require configured local authentication")
	}
	if custodians < 2 {
		return errors.New("bootstrap requires at least two Custodians")
	}
	if majors < 2 {
		return errors.New("bootstrap requires at least two Major-eligible humans")
	}
	if local < 2 {
		return errors.New("bootstrap requires resilient local authentication")
	}
	if len(plan.BreakGlassKeyholderIDs) != 2 {
		return errors.New("bootstrap requires exactly two offline break-glass keyholders")
	}
	if strings.TrimSpace(plan.BreakGlassKeyholderIDs[0]) == "" || strings.TrimSpace(plan.BreakGlassKeyholderIDs[1]) == "" || plan.BreakGlassKeyholderIDs[0] == plan.BreakGlassKeyholderIDs[1] {
		return errors.New("break-glass keyholders must be distinct named humans")
	}
	if _, ok := seen[plan.BreakGlassKeyholderIDs[0]]; !ok {
		return errors.New("break-glass keyholder must be a configured identity")
	}
	if _, ok := seen[plan.BreakGlassKeyholderIDs[1]]; !ok {
		return errors.New("break-glass keyholder must be a configured identity")
	}
	return nil
}

func BootstrapRecords(plan BootstrapPlan, now time.Time) ([]ledger.Record, error) {
	if err := ValidateBootstrap(plan); err != nil {
		return nil, err
	}
	if now.IsZero() {
		return nil, errors.New("bootstrap time is required")
	}
	identities := append([]BootstrapIdentity(nil), plan.Identities...)
	sort.Slice(identities, func(i, j int) bool { return identities[i].HumanID < identities[j].HumanID })
	keyholders := append([]string(nil), plan.BreakGlassKeyholderIDs...)
	sort.Strings(keyholders)

	var records []ledger.Record
	bootstrapPayload := map[string]any{
		"prepared_by":            plan.PreparedBy,
		"confirmed_by":           plan.ConfirmedBy,
		"identity_count":         len(identities),
		"break_glass_keyholders": keyholders,
	}
	record, err := governedSystemRecord(RecordBootstrap, "bootstrap", plan.PreparedBy, plan.ConfirmedBy, bootstrapPayload, now)
	if err != nil {
		return nil, err
	}
	records = append(records, record)
	for _, identity := range identities {
		if identity.MajorEligible {
			r, err := governedSystemRecord(access.RecordRoleEligibility, "role:"+identity.HumanID, plan.PreparedBy, plan.ConfirmedBy, map[string]any{"human_id": identity.HumanID, "role": authz.RoleMajor, "active": true}, now)
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
		if identity.Custodian {
			r, err := governedSystemRecord(access.RecordCustodianDesignation, "custodian:"+identity.HumanID, plan.PreparedBy, plan.ConfirmedBy, map[string]any{"human_id": identity.HumanID, "active": true}, now)
			if err != nil {
				return nil, err
			}
			records = append(records, r)
		}
	}
	return records, nil
}

type RecoveryPlan struct {
	PreparedBy          string
	ConfirmedBy         string
	Reason              string
	RestoreCustodianIDs []string
	PresentedBreakGlass map[string]string
}

func CustodianRecoveryRecords(plan RecoveryPlan, activeBreakGlass []authn.BreakGlassRecord, now time.Time) ([]ledger.Record, error) {
	if strings.TrimSpace(plan.PreparedBy) == "" || strings.TrimSpace(plan.ConfirmedBy) == "" || plan.PreparedBy == plan.ConfirmedBy {
		return nil, errors.New("Custodian recovery requires two distinct named humans")
	}
	if strings.TrimSpace(plan.Reason) == "" {
		return nil, errors.New("Custodian recovery reason is required")
	}
	if now.IsZero() {
		return nil, errors.New("Custodian recovery time is required")
	}
	keyholders, err := authn.VerifyBreakGlass(activeBreakGlass, plan.PresentedBreakGlass)
	if err != nil {
		return nil, fmt.Errorf("Custodian recovery proofs: %w", err)
	}
	restore := uniqueNonBlank(plan.RestoreCustodianIDs)
	if len(restore) < 2 {
		return nil, errors.New("Custodian recovery must restore a quorum of at least two Custodians")
	}
	sort.Strings(restore)
	payload := map[string]any{
		"reason":                 plan.Reason,
		"prepared_by":            plan.PreparedBy,
		"confirmed_by":           plan.ConfirmedBy,
		"break_glass_keyholders": keyholders,
		"restored_custodians":    restore,
	}
	audit, err := governedSystemRecord(RecordCustodianRecovery, "custodian-recovery", plan.PreparedBy, plan.ConfirmedBy, payload, now)
	if err != nil {
		return nil, err
	}
	records := []ledger.Record{audit}
	for _, humanID := range restore {
		r, err := governedSystemRecord(access.RecordCustodianDesignation, "custodian-recovery:"+humanID, plan.PreparedBy, plan.ConfirmedBy, map[string]any{"human_id": humanID, "active": true}, now)
		if err != nil {
			return nil, err
		}
		records = append(records, r)
	}
	return records, nil
}

type BreakGlassCredentialSource interface {
	ActiveBreakGlassRecords(context.Context) ([]authn.BreakGlassRecord, error)
}

type BreakGlassAlert struct {
	HumanID    string
	Keyholders []string
	ExpiresAt  time.Time
}

type BreakGlassAlertSink interface {
	AlertBreakGlass(context.Context, BreakGlassAlert) error
}

type BreakGlassResult struct {
	Token        string
	Session      authn.Session
	Keyholders   []string
	Audit        ledger.Record
	AlertAttempt ledger.Record
}

// EstablishBreakGlass authenticates one existing named human with two offline
// proofs. It calls one alert edge and records the attempt, but notification
// failure never widens authority or cancels otherwise-valid emergency auth.
func EstablishBreakGlass(ctx context.Context, targetHumanID, reason string, presented map[string]string, source BreakGlassCredentialSource, alerts BreakGlassAlertSink, sessions authn.Sessions, now time.Time) (BreakGlassResult, error) {
	if strings.TrimSpace(targetHumanID) == "" || strings.TrimSpace(reason) == "" {
		return BreakGlassResult{}, errors.New("break-glass target human and reason are required")
	}
	if source == nil || alerts == nil || sessions.Store == nil || now.IsZero() {
		return BreakGlassResult{}, errors.New("break-glass security stores, alert edge and time are required")
	}
	records, err := source.ActiveBreakGlassRecords(ctx)
	if err != nil {
		return BreakGlassResult{}, err
	}
	keyholders, err := authn.VerifyBreakGlass(records, presented)
	if err != nil {
		return BreakGlassResult{}, err
	}
	previousNow := sessions.Now
	sessions.Now = func() time.Time { return now }
	token, session, err := sessions.Create(ctx, targetHumanID, "breakglass", true)
	sessions.Now = previousNow
	if err != nil {
		return BreakGlassResult{}, err
	}
	payload := map[string]any{
		"human_id":   targetHumanID,
		"reason":     reason,
		"keyholders": keyholders,
		"expires_at": session.ExpiresAt.UTC(),
	}
	raw, err := json.Marshal(payload)
	if err != nil {
		_ = sessions.Revoke(ctx, token)
		return BreakGlassResult{}, err
	}
	audit := observedSystemRecord(RecordBreakGlass, "break-glass:"+targetHumanID+":"+now.UTC().Format(time.RFC3339Nano), targetHumanID, raw, now)
	if err := ledger.ValidateRecord(audit); err != nil {
		_ = sessions.Revoke(ctx, token)
		return BreakGlassResult{}, err
	}

	alertErr := alerts.AlertBreakGlass(ctx, BreakGlassAlert{HumanID: targetHumanID, Keyholders: append([]string(nil), keyholders...), ExpiresAt: session.ExpiresAt.UTC()})
	outcome := "attempted"
	if alertErr != nil {
		outcome = "failed"
	}
	alertPayload, err := json.Marshal(map[string]any{
		"human_id": targetHumanID,
		"outcome":  outcome,
	})
	if err != nil {
		_ = sessions.Revoke(ctx, token)
		return BreakGlassResult{}, err
	}
	alertAttempt := observedSystemRecord(RecordBreakGlassAlertAttempt, "break-glass-alert:"+targetHumanID+":"+now.UTC().Format(time.RFC3339Nano), targetHumanID, alertPayload, now)
	if err := ledger.ValidateRecord(alertAttempt); err != nil {
		_ = sessions.Revoke(ctx, token)
		return BreakGlassResult{}, err
	}
	return BreakGlassResult{Token: token, Session: session, Keyholders: keyholders, Audit: audit, AlertAttempt: alertAttempt}, nil
}

func observedSystemRecord(recordType, recordID, maker string, payload json.RawMessage, now time.Time) ledger.Record {
	return ledger.Record{
		SchemaVersion:  1,
		Timestamp:      now.UTC(),
		TimeStatus:     ledger.TimeTrusted,
		IncidentMode:   "system",
		WriteSemantic:  "record",
		RecordType:     recordType,
		RecordID:       recordID,
		Status:         "recorded",
		Maker:          maker,
		Compartment:    "SYSTEM",
		Data:           payload,
		MakerSignature: ledger.Signature{MissingReason: "server-observed security event"},
	}
}

func governedSystemRecord(recordType, recordID, maker, confirmer string, payload any, now time.Time) (ledger.Record, error) {
	raw, err := json.Marshal(payload)
	if err != nil {
		return ledger.Record{}, err
	}
	r := ledger.Record{
		SchemaVersion: 1,
		Timestamp:     now.UTC(),
		TimeStatus:    ledger.TimeTrusted,
		IncidentMode:  "system",
		WriteSemantic: "governed",
		RecordType:    recordType,
		RecordID:      recordID,
		Status:        "confirmed",
		Maker:         maker,
		Confirmer:     confirmer,
		Compartment:   "SYSTEM",
		Data:          raw,
	}
	if err := ledger.ValidateRecord(r); err != nil {
		return ledger.Record{}, err
	}
	return r, nil
}

func uniqueNonBlank(values []string) []string {
	seen := map[string]bool{}
	var out []string
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" || seen[value] {
			continue
		}
		seen[value] = true
		out = append(out, value)
	}
	return out
}
