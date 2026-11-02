package access

import "time"

type DesignationView struct {
	HumanID     string
	IncidentID  string
	Compartment string
}

type MembershipView struct {
	HumanID     string
	IncidentID  string
	Compartment string
	EffectiveAt *time.Time
	ExpiresAt   *time.Time
	Reason      string
	Status      string
	Implicit    bool
}

type AdministrationView struct {
	Sponsors               []DesignationView
	Controllers            []DesignationView
	IncidentMemberships    []MembershipView
	CompartmentMemberships []MembershipView
	OutputPermissions      []OutputPermissionView
}

// OutputPermissionView is one confirmed separate output permission.
type OutputPermissionView struct {
	HumanID string
	Action  string
}

func WindowStatus(effectiveAt, expiresAt *time.Time, now time.Time) string {
	if effectiveAt != nil && now.Before(*effectiveAt) {
		return "scheduled"
	}
	if expiresAt != nil && !now.Before(*expiresAt) {
		return "expired"
	}
	return "active"
}

// PersonRecord is the stored state of one person for administration: their
// identity (if recorded), eligibility, Custodian designation and the incidents
// they are currently a member of. It reports state; policy is applied above.
type PersonRecord struct {
	HumanID   string
	Identity  IdentityState
	Legacy    bool
	Role      string
	Custodian bool
	Incidents []string
}
