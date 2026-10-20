package access

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

const (
	RecordRoleEligibility          = "security.role_eligibility"
	RecordCustodianDesignation     = "security.custodian_designation"
	RecordIncidentSponsor          = "security.incident_sponsor"
	RecordCompartmentController    = "security.compartment_controller"
	RecordIncidentMembership       = "security.incident_membership"
	RecordCompartmentMembership    = "security.compartment_membership"
	RecordOutputPermission         = "security.output_permission"
	RecordIsolateMode              = "security.isolate_mode"
)

type IncidentSponsorDesignation struct {
	HumanID    string `json:"human_id"`
	IncidentID string `json:"incident_id"`
	Active     bool   `json:"active"`
}

type CompartmentControllerDesignation struct {
	HumanID     string `json:"human_id"`
	IncidentID  string `json:"incident_id"`
	Compartment string `json:"compartment"`
	Active      bool   `json:"active"`
}

type IncidentMembershipChange struct {
	HumanID     string     `json:"human_id"`
	IncidentID  string     `json:"incident_id"`
	Active      bool       `json:"active"`
	EffectiveAt *time.Time `json:"effective_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

type CompartmentMembershipChange struct {
	HumanID     string     `json:"human_id"`
	IncidentID  string     `json:"incident_id"`
	Compartment string     `json:"compartment"`
	Active      bool       `json:"active"`
	EffectiveAt *time.Time `json:"effective_at,omitempty"`
	ExpiresAt   *time.Time `json:"expires_at,omitempty"`
	Reason      string     `json:"reason,omitempty"`
}

type Change struct {
	RecordType  string
	HumanID     string
	Role        authz.Role
	IncidentID  string
	Compartment string
	Action      authz.Action
	Active      bool
	EffectiveAt *time.Time
	ExpiresAt   *time.Time
	Reason      string
}

func IsRecordType(recordType string) bool {
	switch recordType {
	case RecordRoleEligibility, RecordCustodianDesignation, RecordIncidentSponsor, RecordCompartmentController, RecordIncidentMembership, RecordCompartmentMembership, RecordOutputPermission, RecordIsolateMode:
		return true
	default:
		return false
	}
}

func Decode(recordType string, raw []byte) (Change, bool, error) {
	change := Change{RecordType: recordType}
	switch recordType {
	case RecordRoleEligibility:
		var payload struct {
			HumanID string     `json:"human_id"`
			Role    authz.Role `json:"role"`
			Active  bool       `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || !authz.ValidRole(payload.Role) {
			return Change{}, true, errors.New("invalid role eligibility change")
		}
		change.HumanID, change.Role, change.Active = payload.HumanID, payload.Role, payload.Active
	case RecordCustodianDesignation:
		var payload struct {
			HumanID string `json:"human_id"`
			Active  bool   `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" {
			return Change{}, true, errors.New("invalid Custodian designation change")
		}
		change.HumanID, change.Active = payload.HumanID, payload.Active
	case RecordIncidentSponsor:
		var payload IncidentSponsorDesignation
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" {
			return Change{}, true, errors.New("invalid Incident Sponsor designation")
		}
		change.HumanID, change.IncidentID, change.Active = payload.HumanID, payload.IncidentID, payload.Active
	case RecordCompartmentController:
		var payload CompartmentControllerDesignation
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" || strings.TrimSpace(payload.Compartment) == "" || payload.Compartment == "GENERAL" {
			return Change{}, true, errors.New("invalid Compartment Controller designation")
		}
		change.HumanID, change.IncidentID, change.Compartment, change.Active = payload.HumanID, payload.IncidentID, payload.Compartment, payload.Active
	case RecordIncidentMembership:
		var payload IncidentMembershipChange
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" {
			return Change{}, true, errors.New("invalid incident membership change")
		}
		if err := validateWindow(payload.Active, payload.EffectiveAt, payload.ExpiresAt); err != nil {
			return Change{}, true, err
		}
		change.HumanID, change.IncidentID, change.Active = payload.HumanID, payload.IncidentID, payload.Active
		change.EffectiveAt, change.ExpiresAt, change.Reason = payload.EffectiveAt, payload.ExpiresAt, payload.Reason
	case RecordCompartmentMembership:
		var payload CompartmentMembershipChange
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" || strings.TrimSpace(payload.Compartment) == "" || payload.Compartment == "GENERAL" {
			return Change{}, true, errors.New("invalid restricted compartment membership change")
		}
		if err := validateWindow(payload.Active, payload.EffectiveAt, payload.ExpiresAt); err != nil {
			return Change{}, true, err
		}
		change.HumanID, change.IncidentID, change.Compartment, change.Active = payload.HumanID, payload.IncidentID, payload.Compartment, payload.Active
		change.EffectiveAt, change.ExpiresAt, change.Reason = payload.EffectiveAt, payload.ExpiresAt, payload.Reason
	case RecordOutputPermission:
		var payload struct {
			HumanID string       `json:"human_id"`
			Action  authz.Action `json:"action"`
			Active  bool         `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || !authz.IsSeparateOutput(payload.Action) {
			return Change{}, true, errors.New("invalid output permission change")
		}
		change.HumanID, change.Action, change.Active = payload.HumanID, payload.Action, payload.Active
	case RecordIsolateMode:
		var payload struct {
			Active bool `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		change.Active = payload.Active
	default:
		return Change{}, false, nil
	}
	return change, true, nil
}

func validateWindow(active bool, effectiveAt, expiresAt *time.Time) error {
	if !active && (effectiveAt != nil || expiresAt != nil) {
		return errors.New("revocation cannot carry an access window")
	}
	if effectiveAt != nil && effectiveAt.IsZero() {
		return errors.New("access effective time cannot be zero")
	}
	if expiresAt != nil && expiresAt.IsZero() {
		return errors.New("access expiry cannot be zero")
	}
	if effectiveAt != nil && expiresAt != nil && !expiresAt.After(*effectiveAt) {
		return errors.New("access expiry must be after effective time")
	}
	return nil
}

func decodeStrict(raw []byte, target any) error {
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(target); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return errors.New("multiple JSON values")
		}
		return err
	}
	return nil
}
