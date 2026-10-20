package authz

import "strings"

type Role string

const (
	RoleMajor    Role = "Major"
	RoleMinor    Role = "Minor"
	RoleObserver Role = "Observer"
)

type Action string

const (
	ActionRead                  Action = "read"
	ActionSearch                Action = "search"
	ActionComment               Action = "comment"
	ActionPropose               Action = "propose"
	ActionConfirm               Action = "confirm"
	ActionDownload              Action = "download"
	ActionPrint                 Action = "print"
	ActionExport                Action = "export"
	ActionAPI                   Action = "api"
	ActionMCP                   Action = "mcp"
	ActionModelSend             Action = "model_send"
	ActionNotify                Action = "notify"
	ActionBackup                Action = "backup"
	ActionAdminIdentityMetadata Action = "admin_identity_metadata"
)

type Principal struct {
	Authenticated bool
	HumanID       string
	AgentID       string
	Role          Role
	Custodian     bool
	Incidents     map[string]bool
	Compartments  map[string]map[string]bool
	Outputs       map[Action]bool
}

type ObjectScope struct {
	IncidentID     string
	Compartment    string
	SystemMetadata bool
}

type Decision struct {
	Allowed bool
	Reason  string
}

// Authorise is CRAB's single application authorisation decision. The caller
// must resolve an object's authoritative incident/compartment before calling it.
func Authorise(p Principal, action Action, scope ObjectScope) Decision {
	deny := func(reason string) Decision { return Decision{Reason: reason} }
	if !p.Authenticated || strings.TrimSpace(p.HumanID) == "" {
		return deny("authentication required")
	}

	if action == ActionAdminIdentityMetadata {
		if p.Custodian && scope.SystemMetadata {
			return Decision{Allowed: true}
		}
		return deny("custodian metadata authority required")
	}

	if scope.SystemMetadata || strings.TrimSpace(scope.IncidentID) == "" {
		return deny("incident scope required")
	}
	if !p.Incidents[scope.IncidentID] {
		return deny("incident membership required")
	}
	compartment := scope.Compartment
	if compartment == "" {
		compartment = "GENERAL"
	}
	if !hasCompartment(p, scope.IncidentID, compartment) {
		return deny("compartment membership required")
	}
	if !rolePermits(p.Role, action) {
		return deny("role does not permit action")
	}
	if isSeparateOutput(action) && !p.Outputs[action] {
		return deny("output/action permission required")
	}
	return Decision{Allowed: true}
}

func hasCompartment(p Principal, incidentID, compartment string) bool {
	byIncident := p.Compartments[incidentID]
	return byIncident != nil && byIncident[compartment]
}

func rolePermits(role Role, action Action) bool {
	switch action {
	case ActionRead, ActionSearch, ActionDownload, ActionPrint, ActionExport, ActionAPI, ActionMCP, ActionModelSend:
		return role == RoleMajor || role == RoleMinor || role == RoleObserver
	case ActionComment, ActionNotify:
		return role == RoleMajor || role == RoleMinor
	case ActionPropose, ActionConfirm, ActionBackup:
		return role == RoleMajor
	default:
		return false
	}
}

func isSeparateOutput(action Action) bool {
	switch action {
	case ActionDownload, ActionPrint, ActionExport, ActionAPI, ActionMCP, ActionModelSend, ActionNotify, ActionBackup:
		return true
	default:
		return false
	}
}
