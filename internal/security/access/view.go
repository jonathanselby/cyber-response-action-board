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
	Controllers             []DesignationView
	IncidentMemberships     []MembershipView
	CompartmentMemberships  []MembershipView
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
