package ceremony

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"errors"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
)

type memorySessions struct {
	values map[[32]byte]authn.Session
}

func (m *memorySessions) PutSession(_ context.Context, digest [32]byte, session authn.Session) error {
	if m.values == nil {
		m.values = map[[32]byte]authn.Session{}
	}
	m.values[digest] = session
	return nil
}
func (m *memorySessions) GetSession(_ context.Context, digest [32]byte, now time.Time) (authn.Session, bool, error) {
	s, ok := m.values[digest]
	return s, ok && now.Before(s.ExpiresAt), nil
}
func (m *memorySessions) DeleteSession(_ context.Context, digest [32]byte) error {
	delete(m.values, digest)
	return nil
}

type memoryBreakGlass []authn.BreakGlassRecord

func (m memoryBreakGlass) ActiveBreakGlassRecords(context.Context) ([]authn.BreakGlassRecord, error) {
	return []authn.BreakGlassRecord(m), nil
}

type memoryAlert struct {
	called bool
	fail   bool
	alert  BreakGlassAlert
}

func (m *memoryAlert) AlertBreakGlass(_ context.Context, alert BreakGlassAlert) error {
	m.called = true
	m.alert = alert
	if m.fail {
		return errors.New("injected alert failure")
	}
	return nil
}

func validBootstrap() BootstrapPlan {
	return BootstrapPlan{
		PreparedBy:  "alice",
		ConfirmedBy: "bob",
		Identities: []BootstrapIdentity{
			{HumanID: "alice", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
			{HumanID: "bob", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
		},
		BreakGlassKeyholderIDs: []string{"alice", "bob"},
	}
}

func TestBootstrapCreatesNoIncidentVisibility(t *testing.T) {
	plan := validBootstrap()
	records, err := BootstrapRecords(plan, time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 5 {
		t.Fatalf("bootstrap records=%d", len(records))
	}
	for _, record := range records {
		if record.IncidentID != "" || record.IncidentMode != "system" {
			t.Fatalf("bootstrap created incident scope: %+v", record)
		}
		if record.RecordType == "security.incident_membership" || record.RecordType == "security.compartment_membership" {
			t.Fatalf("bootstrap created content visibility: %s", record.RecordType)
		}
	}
}

func TestBootstrapRejectsOnePersonAndInsufficientCustodians(t *testing.T) {
	plan := validBootstrap()
	plan.ConfirmedBy = plan.PreparedBy
	if err := ValidateBootstrap(plan); err == nil {
		t.Fatal("one-person bootstrap accepted")
	}
	plan = validBootstrap()
	plan.Identities[1].Custodian = false
	if err := ValidateBootstrap(plan); err == nil {
		t.Fatal("bootstrap with one Custodian accepted")
	}
}

func TestCustodianRecoveryRestoresCustodiansOnly(t *testing.T) {
	a, _ := authn.NewBreakGlassRecord("key-a", "offline-a", authn.MinPBKDF2Iterations)
	b, _ := authn.NewBreakGlassRecord("key-b", "offline-b", authn.MinPBKDF2Iterations)
	records, err := CustodianRecoveryRecords(RecoveryPlan{
		PreparedBy:          "alice",
		ConfirmedBy:         "bob",
		Reason:              "Custodian quorum lost",
		RestoreCustodianIDs: []string{"alice", "bob"},
		PresentedBreakGlass: map[string]string{"key-a": "offline-a", "key-b": "offline-b"},
	}, []authn.BreakGlassRecord{a, b}, time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC))
	if err != nil {
		t.Fatal(err)
	}
	if len(records) != 3 {
		t.Fatalf("recovery records=%d", len(records))
	}
	for _, record := range records {
		if record.IncidentID != "" || record.IncidentMode != "system" {
			t.Fatalf("recovery widened incident scope: %+v", record)
		}
		if record.RecordType != RecordCustodianRecovery && record.RecordType != "security.custodian_designation" {
			t.Fatalf("recovery created forbidden authority: %s", record.RecordType)
		}
	}
}

func TestBreakGlassCreatesAuthenticationSessionWithoutAuthority(t *testing.T) {
	a, _ := authn.NewBreakGlassRecord("key-a", "offline-a", authn.MinPBKDF2Iterations)
	b, _ := authn.NewBreakGlassRecord("key-b", "offline-b", authn.MinPBKDF2Iterations)
	store := &memorySessions{}
	alerts := &memoryAlert{}
	now := time.Date(2026, 10, 20, 4, 0, 0, 0, time.UTC)
	result, err := EstablishBreakGlass(context.Background(), "responder-1", "corporate identity unavailable", map[string]string{"key-a": "offline-a", "key-b": "offline-b"}, memoryBreakGlass{a, b}, alerts, authn.Sessions{Store: store, TTL: 15 * time.Minute}, now)
	if err != nil {
		t.Fatal(err)
	}
	if !result.Session.BreakGlass || result.Session.HumanID != "responder-1" || result.Audit.IncidentID != "" {
		t.Fatalf("break-glass result=%+v audit=%+v", result.Session, result.Audit)
	}
	if !alerts.called || alerts.alert.HumanID != "responder-1" || result.AlertAttempt.RecordType != RecordBreakGlassAlertAttempt {
		t.Fatalf("alert attempt missing called=%v alert=%+v record=%+v", alerts.called, alerts.alert, result.AlertAttempt)
	}
	raw, err := base64.RawURLEncoding.DecodeString(result.Token)
	if err != nil || len(raw) != authn.SessionTokenBytes {
		t.Fatalf("invalid returned bearer token err=%v bytes=%d", err, len(raw))
	}
	digest := sha256.Sum256(raw)
	stored, ok := store.values[digest]
	if !ok || stored.HumanID != "responder-1" {
		t.Fatal("break-glass bearer did not map to existing named human")
	}
	if len(result.Keyholders) != 2 {
		t.Fatal("break-glass did not retain two-keyholder provenance")
	}
}

func TestBreakGlassAlertFailureDoesNotCancelAuthentication(t *testing.T) {
	a, _ := authn.NewBreakGlassRecord("key-a", "offline-a", authn.MinPBKDF2Iterations)
	b, _ := authn.NewBreakGlassRecord("key-b", "offline-b", authn.MinPBKDF2Iterations)
	store := &memorySessions{}
	alerts := &memoryAlert{fail: true}
	now := time.Date(2026, 10, 20, 4, 30, 0, 0, time.UTC)
	result, err := EstablishBreakGlass(context.Background(), "responder-1", "corporate identity unavailable", map[string]string{"key-a": "offline-a", "key-b": "offline-b"}, memoryBreakGlass{a, b}, alerts, authn.Sessions{Store: store, TTL: 15 * time.Minute}, now)
	if err != nil {
		t.Fatalf("alert failure cancelled break-glass authentication: %v", err)
	}
	if result.Token == "" || result.AlertAttempt.RecordType != RecordBreakGlassAlertAttempt {
		t.Fatalf("break-glass result missing token/alert audit: %+v", result)
	}
	if string(result.AlertAttempt.Data) != `{"human_id":"responder-1","outcome":"failed"}` {
		t.Fatalf("failed alert outcome not recorded: %s", result.AlertAttempt.Data)
	}
}
