package access

import (
	"errors"
	"testing"
	"time"
)

func TestUsableIsTheSingleAccountRule(t *testing.T) {
	now := time.Date(2026, 11, 2, 2, 17, 0, 0, time.UTC)
	future, past := now.Add(time.Hour), now.Add(-time.Second)
	cases := []struct {
		name   string
		state  IdentityState
		legacy bool
		want   bool
	}{
		{"no identity, not legacy", IdentityState{}, false, false},
		{"no identity, legacy eligibility", IdentityState{}, true, true},
		{"active", IdentityState{Found: true, Status: IdentityActive}, false, true},
		{"active, legacy flag ignored once recorded", IdentityState{Found: true, Status: IdentitySuspended}, true, false},
		{"active with future expiry", IdentityState{Found: true, Status: IdentityActive, AccountExpiresAt: &future}, false, true},
		{"active but expired", IdentityState{Found: true, Status: IdentityActive, AccountExpiresAt: &past}, false, false},
		{"expiry exactly now", IdentityState{Found: true, Status: IdentityActive, AccountExpiresAt: &now}, false, false},
		{"suspended", IdentityState{Found: true, Status: IdentitySuspended}, false, false},
		{"disabled", IdentityState{Found: true, Status: IdentityDisabled}, false, false},
	}
	for _, c := range cases {
		if got := Usable(c.state, c.legacy, now); got != c.want {
			t.Errorf("%s: Usable=%v want %v", c.name, got, c.want)
		}
	}
}

func TestIdentityTransitionRules(t *testing.T) {
	now := time.Date(2026, 11, 2, 2, 17, 0, 0, time.UTC)
	week, month, past := now.Add(7*24*time.Hour), now.Add(31*24*time.Hour), now.Add(-time.Minute)
	base := IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Status: IdentityActive}
	external := base
	external.External, external.AccountExpiresAt = true, &week
	if err := ValidateIdentityTransition(external, IdentityState{}, now); err != nil {
		t.Fatalf("valid external: %v", err)
	}
	noExpiry := base
	noExpiry.External = true
	if err := ValidateIdentityTransition(noExpiry, IdentityState{}, now); !errors.Is(err, ErrExternalNeedsExpiry) {
		t.Fatalf("external without expiry err=%v", err)
	}
	tooLong := external
	tooLong.AccountExpiresAt = &month
	if err := ValidateIdentityTransition(tooLong, IdentityState{}, now); !errors.Is(err, ErrAccountExpiryTooLong) {
		t.Fatalf("external beyond 30 days err=%v", err)
	}
	inPast := base
	inPast.AccountExpiresAt = &past
	if err := ValidateIdentityTransition(inPast, IdentityState{}, now); !errors.Is(err, ErrAccountExpiryInPast) {
		t.Fatalf("expiry in past err=%v", err)
	}
	if err := ValidateIdentityTransition(base, IdentityState{Found: true, Status: IdentityDisabled}, now); !errors.Is(err, ErrIdentityDisabled) {
		t.Fatalf("change after disable err=%v", err)
	}
	suspendNoReason := base
	suspendNoReason.Status = IdentitySuspended
	if err := ValidateIdentityTransition(suspendNoReason, IdentityState{Found: true, Status: IdentityActive}, now); err == nil {
		t.Fatal("suspension without a reason accepted")
	}
	noName := base
	noName.DisplayName = "  "
	if err := ValidateIdentityTransition(noName, IdentityState{}, now); err == nil {
		t.Fatal("identity without a display name accepted")
	}
}

func TestIdentityAndResetDecodeStrictly(t *testing.T) {
	change, recognised, err := Decode(RecordIdentity, []byte(`{"human_id":"jane","display_name":"Jane","external":false,"status":"active"}`))
	if err != nil || !recognised || change.Identity == nil || !change.Active {
		t.Fatalf("identity decode=%+v recognised=%v err=%v", change, recognised, err)
	}
	if _, _, err := Decode(RecordIdentity, []byte(`{"human_id":"jane","display_name":"Jane","external":false,"status":"active","phone":"+61"}`)); err == nil {
		t.Fatal("identity accepted an unknown field (contact data must stay out of the ledger)")
	}
	if _, _, err := Decode(RecordCredentialReset, []byte(`{"human_id":"jane","reason":""}`)); err == nil {
		t.Fatal("credential reset accepted without a reason")
	}
}
