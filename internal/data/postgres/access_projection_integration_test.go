package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/json"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

type accessKeys struct {
	keyring       ledger.Keyring
	makerPriv     ed25519.PrivateKey
	confirmerPriv ed25519.PrivateKey
	serverPriv    ed25519.PrivateKey
}

func newAccessKeys(t *testing.T) accessKeys {
	t.Helper()
	makerPub, makerPriv, _ := ed25519.GenerateKey(rand.Reader)
	confirmerPub, confirmerPriv, _ := ed25519.GenerateKey(rand.Reader)
	serverPub, serverPriv, _ := ed25519.GenerateKey(rand.Reader)
	return accessKeys{
		keyring: ledger.Keyring{
			"maker":     base64.StdEncoding.EncodeToString(makerPub),
			"confirmer": base64.StdEncoding.EncodeToString(confirmerPub),
			"server":    base64.StdEncoding.EncodeToString(serverPub),
		},
		makerPriv: makerPriv, confirmerPriv: confirmerPriv, serverPriv: serverPriv,
	}
}

func confirmedAccessRecord(t *testing.T, keys accessKeys, recordType, recordID, incidentID, mode, compartment string, payload any) ledger.Record {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	record := ledger.Record{
		SchemaVersion: 1,
		Timestamp:     time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC),
		TimeStatus:    "trusted",
		IncidentID:    incidentID,
		IncidentMode:  mode,
		WriteSemantic: "governed",
		RecordType:    recordType,
		RecordID:      recordID,
		Status:        "confirmed",
		Maker:         "maker",
		Confirmer:     "confirmer",
		Compartment:   compartment,
		Data:          raw,
	}
	makerSig, err := ledger.SignRecord(record, keys.makerPriv)
	if err != nil {
		t.Fatal(err)
	}
	record.MakerSignature = ledger.Signature{KeyID: "maker", Value: makerSig}
	confirmerSig, err := ledger.SignRecord(record, keys.confirmerPriv)
	if err != nil {
		t.Fatal(err)
	}
	record.ConfirmerSignature = ledger.Signature{KeyID: "confirmer", Value: confirmerSig}
	return record
}

func TestAccessProjectionAndPrincipalAreDenyByDefault(t *testing.T) {
	db := setupTestDB(t)
	ctx := context.Background()
	keys := newAccessKeys(t)
	kernel, err := OpenKernel(ctx, db.runtime, keys.keyring)
	if err != nil {
		t.Fatal(err)
	}
	projection := AccessProjection{}
	signer := Signer{KeyID: "server", PrivateKey: keys.serverPriv}
	appendChange := func(record ledger.Record) {
		t.Helper()
		if _, err := kernel.Append(ctx, record, signer, projection); err != nil {
			t.Fatal(err)
		}
	}

	appendChange(confirmedAccessRecord(t, keys, access.RecordRoleEligibility, "role-1", "", "system", "SYSTEM", map[string]any{"human_id": "human-1", "role": "Major", "active": true}))
	appendChange(confirmedAccessRecord(t, keys, access.RecordCustodianDesignation, "custodian-1", "", "system", "SYSTEM", map[string]any{"human_id": "human-1", "active": true}))
	appendChange(confirmedAccessRecord(t, keys, access.RecordIncidentMembership, "incident-access-1", "INC-A", "live", "GENERAL", map[string]any{"human_id": "human-1", "incident_id": "INC-A", "active": true}))
	appendChange(confirmedAccessRecord(t, keys, access.RecordCompartmentMembership, "compartment-1", "INC-A", "live", "LEGAL", map[string]any{"human_id": "human-1", "incident_id": "INC-A", "compartment": "LEGAL", "active": true}))
	appendChange(confirmedAccessRecord(t, keys, access.RecordOutputPermission, "output-1", "", "system", "SYSTEM", map[string]any{"human_id": "human-1", "action": "export", "active": true}))

	store, err := NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	principal, err := store.Principal(ctx, "human-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if principal.Role != authz.RoleMajor || !principal.Custodian || !principal.Incidents["INC-A"] || !principal.Compartments["INC-A"]["GENERAL"] || !principal.Compartments["INC-A"]["LEGAL"] || !principal.Outputs[authz.ActionExport] {
		t.Fatalf("principal projection=%+v", principal)
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: "INC-A", Compartment: "GENERAL"}); !d.Allowed {
		t.Fatalf("assigned incident denied: %+v", d)
	}
	if d := authz.Authorise(principal, authz.ActionRead, authz.ObjectScope{IncidentID: "INC-B", Compartment: "GENERAL"}); d.Allowed {
		t.Fatalf("Major gained universal visibility: %+v", d)
	}
	if d := authz.Authorise(principal, authz.ActionExport, authz.ObjectScope{IncidentID: "INC-A", Compartment: "LEGAL"}); !d.Allowed {
		t.Fatalf("separately permitted export denied: %+v", d)
	}

	if _, err := kernel.Rebuild(ctx, projection); err != nil {
		t.Fatal(err)
	}
	rebuilt, err := store.Principal(ctx, "human-1", true)
	if err != nil || !rebuilt.Incidents["INC-A"] || !rebuilt.Compartments["INC-A"]["LEGAL"] {
		t.Fatalf("rebuilt principal=%+v err=%v", rebuilt, err)
	}

	appendChange(confirmedAccessRecord(t, keys, access.RecordIncidentMembership, "incident-access-2", "INC-A", "live", "GENERAL", map[string]any{"human_id": "human-1", "incident_id": "INC-A", "active": false}))
	revoked, err := store.Principal(ctx, "human-1", true)
	if err != nil {
		t.Fatal(err)
	}
	if revoked.Incidents["INC-A"] || revoked.Compartments["INC-A"] != nil {
		t.Fatalf("incident revocation left access behind: %+v", revoked)
	}
}
