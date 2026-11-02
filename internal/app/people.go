package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// MinimumUsableCustodians is the recommended number of usable Custodians, so
// dual control survives one being unavailable (U-15). Fewer is a warning only.
const MinimumUsableCustodians = 3

// PersonView is one person as a Custodian sees them in access administration.
// Incident names appear only where the Custodian may see that incident; the
// rest are counted (U-17).
type PersonView struct {
	HumanID          string
	DisplayName      string
	Organisation     string
	External         bool
	Status           string // active, suspended, disabled, or legacy
	AccountExpiresAt *time.Time
	Role             string
	HoldsCustodian   bool
	Usable           bool
	Legacy           bool
	VisibleIncidents []string
	OtherIncidents   int
	// What the viewing Custodian may propose for this person now.
	CanSuspend, CanReactivate, CanDisable, CanReset, CanRecordIdentity, CanChangeRole bool
}

// People returns everyone known to access administration, for a usable
// Custodian only.
func (s AccessService) People(ctx context.Context, principal authz.Principal, now time.Time) ([]PersonView, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return nil, err
	}
	records, err := s.Backend.People(ctx, now)
	if err != nil {
		return nil, err
	}
	out := make([]PersonView, 0, len(records))
	for _, r := range records {
		v := PersonView{HumanID: r.HumanID, Role: r.Role, HoldsCustodian: r.Custodian, Legacy: r.Legacy, Usable: access.Usable(r.Identity, r.Legacy, now)}
		if r.Identity.Found {
			v.DisplayName, v.Organisation, v.External, v.Status, v.AccountExpiresAt = r.Identity.DisplayName, r.Identity.Organisation, r.Identity.External, r.Identity.Status, r.Identity.AccountExpiresAt
		} else {
			v.DisplayName, v.Status = r.HumanID, "legacy"
		}
		for _, id := range r.Incidents {
			if principal.Incidents[id] {
				v.VisibleIncidents = append(v.VisibleIncidents, id)
			} else {
				v.OtherIncidents++
			}
		}
		v.CanSuspend = r.Identity.Found && r.Identity.Status == access.IdentityActive
		v.CanReactivate = r.Identity.Found && r.Identity.Status == access.IdentitySuspended
		v.CanDisable = r.Identity.Found && r.Identity.Status != access.IdentityDisabled
		v.CanReset = v.Usable
		v.CanRecordIdentity = !r.Identity.Found
		v.CanChangeRole = r.Identity.Found && r.Identity.Status == access.IdentityActive
		out = append(out, v)
	}
	return out, nil
}

// ProposeNewPerson proposes a new person's identity and role eligibility as
// one step. Both are validated before either is written, so a refusal never
// leaves half a person pending. The record IDs share the given prefix so
// Approvals can group them; the identity must be confirmed first.
func (s AccessService) ProposeNewPerson(ctx context.Context, principal authz.Principal, prefix string, change access.IdentityChange, role authz.Role, now time.Time) error {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return err
	}
	if !authz.ValidRole(role) {
		return fmt.Errorf("choose a role: Major, Minor or Observer")
	}
	change.Status = access.IdentityActive
	current, _, err := s.Backend.IdentityState(ctx, strings.TrimSpace(change.HumanID))
	if err != nil {
		return err
	}
	if current.Found {
		return fmt.Errorf("a person with the ID %q already exists", strings.TrimSpace(change.HumanID))
	}
	if err := access.ValidateIdentityTransition(change, current, now); err != nil {
		return err
	}
	if _, err := s.ProposeIdentity(ctx, principal, prefix+"-IDENTITY", change, now); err != nil {
		return err
	}
	_, err = s.ProposeRoleEligibility(ctx, principal, prefix+"-ROLE", change.HumanID, role, true, now)
	return err
}

// ProposeIdentityStatus proposes suspending, reactivating or disabling a
// person, keeping the rest of their identity as recorded.
func (s AccessService) ProposeIdentityStatus(ctx context.Context, principal authz.Principal, recordID, humanID, status, reason string, now time.Time) (ledger.Record, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, err
	}
	current, _, err := s.Backend.IdentityState(ctx, strings.TrimSpace(humanID))
	if err != nil {
		return ledger.Record{}, err
	}
	if !current.Found {
		return ledger.Record{}, access.ErrIdentityRecordMissing
	}
	change := access.IdentityChange{HumanID: strings.TrimSpace(humanID), DisplayName: current.DisplayName, Organisation: current.Organisation, External: current.External, Status: status, AccountExpiresAt: current.AccountExpiresAt, Reason: reason}
	if status == access.IdentityActive && current.External && (current.AccountExpiresAt == nil || !current.AccountExpiresAt.After(now)) {
		return ledger.Record{}, fmt.Errorf("%w: record a new account expiry to reactivate an external person", access.ErrAccountExpiryInPast)
	}
	return s.ProposeIdentity(ctx, principal, recordID, change, now)
}

// CustodianReadiness reports how many Custodians are usable now. Fewer than
// three is a warning; CRAB keeps operating (U-15).
func (s AccessService) CustodianReadiness(ctx context.Context, now time.Time) (Check, error) {
	if err := s.ready(); err != nil {
		return Check{}, err
	}
	records, err := s.Backend.People(ctx, now)
	if err != nil {
		return Check{}, err
	}
	usable := 0
	for _, r := range records {
		if r.Custodian && access.Usable(r.Identity, r.Legacy, now) {
			usable++
		}
	}
	check := Check{Name: "custodians", Status: "pass", Message: fmt.Sprintf("%d usable Custodians; dual control survives one being unavailable", usable)}
	if usable < MinimumUsableCustodians {
		check.Status = "warn"
		check.Message = fmt.Sprintf("%d usable Custodian(s); at least %d are recommended so identity changes still have a second Custodian when one is unavailable", usable, MinimumUsableCustodians)
	}
	return check, nil
}

// ExternalPeople reports which people are external, so views can mark them in
// words next to names the viewer already sees. It reveals nothing else.
func (s AccessService) ExternalPeople(ctx context.Context) (map[string]bool, error) {
	if err := s.ready(); err != nil {
		return nil, err
	}
	return s.Backend.ExternalPeople(ctx)
}

// AccessApprovalSummary words an identity, eligibility or credential-reset
// proposal for the Approvals screen. ok=false for other record types.
func AccessApprovalSummary(recordType string, payload map[string]any) (summary, consequence string, ok bool) {
	str := func(k string) string { s, _ := payload[k].(string); return s }
	who := str("human_id")
	switch recordType {
	case access.RecordIdentity:
		name, status := str("display_name"), str("status")
		label := name
		if org := str("organisation"); org != "" {
			label += " (" + org + ")"
		}
		if ext, _ := payload["external"].(bool); ext {
			label += " · EXTERNAL"
		}
		expiry := ""
		if e := str("account_expires_at"); e != "" {
			if t, err := time.Parse(time.RFC3339, e); err == nil {
				expiry = " Their account ends at " + t.UTC().Format("2 Jan 2006 15:04 UTC") + "."
			}
		}
		switch status {
		case access.IdentitySuspended:
			return "Suspend " + label + " [" + who + "]", "They are signed out everywhere at once and cannot sign in until reactivated. History is kept.", true
		case access.IdentityDisabled:
			return "Disable " + label + " [" + who + "]", "They are signed out and can never sign in again; their credentials are removed. This cannot be undone. History is kept.", true
		default:
			return "Record " + label + " as an active person [" + who + "]", "They can sign in once they have a role and an enrolment code." + expiry, true
		}
	case access.RecordRoleEligibility:
		role := str("role")
		if active, _ := payload["active"].(bool); !active {
			return "Remove " + who + "'s CRAB role", "They keep their identity but can no longer act in any incident.", true
		}
		return "Make " + who + " eligible as " + role, role + " lets them work in incidents they are later given access to. It grants no incident access by itself.", true
	case access.RecordCredentialReset:
		return "Issue an enrolment code to " + who, "Their sessions, password and authenticator are reset. The code is shown once to you, the confirmer, to pass to them directly. Reason: " + str("reason"), true
	}
	return "", "", false
}
