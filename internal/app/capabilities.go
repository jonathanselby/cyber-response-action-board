package app

import "github.com/jonathanselby/cyber-response-action-board/internal/security/authz"

// Capabilities are what a person may be offered in the browser. They are
// hints for what to show: every action is still authorised where it is
// performed. Deciding them here keeps role policy out of the composition and
// presentation tiers (F-7, §3.17).
type Capabilities struct {
	Role string
	// Custodian administers people and access metadata.
	IsCustodian bool
	// Major-only areas.
	CanDeclare, CanTriage, CanProposeOrganisation, CanManageObligationLibrary bool
	// CanReadContacts sees the organisation's emergency contacts.
	CanReadContacts bool
}

// CapabilitiesOf returns a person's capabilities.
func CapabilitiesOf(p authz.Principal) Capabilities {
	major := p.Authenticated && p.Role == authz.RoleMajor
	return Capabilities{
		Role: string(p.Role), IsCustodian: p.Authenticated && p.Custodian,
		CanDeclare: major, CanTriage: major, CanProposeOrganisation: major, CanManageObligationLibrary: major,
		CanReadContacts: major || (p.Authenticated && p.Custodian),
	}
}
