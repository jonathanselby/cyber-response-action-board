package access

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"strings"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

const (
	RecordRoleEligibility       = "security.role_eligibility"
	RecordCustodianDesignation  = "security.custodian_designation"
	RecordIncidentMembership    = "security.incident_membership"
	RecordCompartmentMembership = "security.compartment_membership"
	RecordOutputPermission      = "security.output_permission"
)

type Change struct {
	RecordType  string
	HumanID     string
	Role        authz.Role
	IncidentID  string
	Compartment string
	Action      authz.Action
	Active      bool
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
	case RecordIncidentMembership:
		var payload struct {
			HumanID    string `json:"human_id"`
			IncidentID string `json:"incident_id"`
			Active     bool   `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" {
			return Change{}, true, errors.New("invalid incident membership change")
		}
		change.HumanID, change.IncidentID, change.Active = payload.HumanID, payload.IncidentID, payload.Active
	case RecordCompartmentMembership:
		var payload struct {
			HumanID    string `json:"human_id"`
			IncidentID string `json:"incident_id"`
			Compartment string `json:"compartment"`
			Active     bool   `json:"active"`
		}
		if err := decodeStrict(raw, &payload); err != nil {
			return Change{}, true, err
		}
		if strings.TrimSpace(payload.HumanID) == "" || strings.TrimSpace(payload.IncidentID) == "" || strings.TrimSpace(payload.Compartment) == "" || payload.Compartment == "GENERAL" {
			return Change{}, true, errors.New("invalid restricted compartment membership change")
		}
		change.HumanID, change.IncidentID, change.Compartment, change.Active = payload.HumanID, payload.IncidentID, payload.Compartment, payload.Active
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
	default:
		return Change{}, false, nil
	}
	return change, true, nil
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
