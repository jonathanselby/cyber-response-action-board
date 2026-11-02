package access

import (
	"errors"
	"testing"
	"time"
)

func TestIntakeKeyChangeRules(t *testing.T) {
	now := time.Date(2026, 11, 2, 3, 0, 0, 0, time.UTC)
	in := func(d time.Duration) *time.Time { v := now.Add(d); return &v }
	active := IntakeKeyState{Found: true, KeyName: "siem", AccountableHuman: "alice", Status: IntakeKeyActive, ExpiresAt: now.Add(time.Hour)}
	expired := active
	expired.ExpiresAt = now.Add(-time.Second)
	revoked := active
	revoked.Status = IntakeKeyRevoked
	for _, tc := range []struct {
		name    string
		change  IntakeKeyChange
		current IntakeKeyState
		want    error
	}{
		{"issue", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyIssue, AccountableHuman: "alice", ExpiresAt: in(24 * time.Hour), Reason: "r"}, IntakeKeyState{}, nil},
		{"issue reuses a name", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyIssue, AccountableHuman: "alice", ExpiresAt: in(24 * time.Hour), Reason: "r"}, revoked, ErrIntakeKeyExists},
		{"issue without expiry", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyIssue, AccountableHuman: "alice", Reason: "r"}, IntakeKeyState{}, ErrIntakeKeyNeedsExpiry},
		{"issue too far", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyIssue, AccountableHuman: "alice", ExpiresAt: in(366 * 24 * time.Hour), Reason: "r"}, IntakeKeyState{}, ErrIntakeKeyExpiryTooFar},
		{"issue in past", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyIssue, AccountableHuman: "alice", ExpiresAt: in(-time.Minute), Reason: "r"}, IntakeKeyState{}, ErrIntakeKeyNeedsExpiry},
		{"bad name", IntakeKeyChange{KeyName: "Siem Prod", Action: IntakeKeyIssue, AccountableHuman: "alice", ExpiresAt: in(time.Hour), Reason: "r"}, IntakeKeyState{}, ErrIntakeKeyName},
		{"no reason", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRevoke}, active, ErrIntakeKeyReason},
		{"rotate 24h", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRotate, OverlapMinutes: 24 * 60, Reason: "r"}, active, nil},
		{"rotate 24h01", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRotate, OverlapMinutes: 24*60 + 1, Reason: "r"}, active, ErrIntakeKeyOverlap},
		{"rotate expired", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRotate, Reason: "r"}, expired, ErrIntakeKeyNotActive},
		{"revoke revoked", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRevoke, Reason: "r"}, revoked, ErrIntakeKeyNotActive},
		{"revoke", IntakeKeyChange{KeyName: "siem", Action: IntakeKeyRevoke, Reason: "r"}, active, nil},
	} {
		if err := ValidateIntakeKeyChange(tc.change, tc.current, now); !errors.Is(err, tc.want) && !(tc.want == nil && err == nil) {
			t.Errorf("%s: err=%v want %v", tc.name, err, tc.want)
		}
	}
	if _, _, err := Decode(RecordIntakeKey, []byte(`{"key_name":"siem","action":"rotate","accountable_human":"mallory","reason":"r"}`)); err == nil {
		t.Error("a rotation that changes the accountable person decoded")
	}
}
