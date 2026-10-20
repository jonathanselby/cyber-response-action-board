package main

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/ceremony"
)

const operatorSessionTTL = 8 * time.Hour

type operatorRuntime struct {
	kernel    *postgres.Kernel
	signer    postgres.Signer
	security  *postgres.SecurityStore
	access    *postgres.AccessStore
	incidents *postgres.IncidentStore
	incident  app.IncidentService
	local     authn.Local
	sessions  authn.Sessions
}

type signingKeyFile struct {
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
}

func init() {
	web.RuntimeFactory = newOperatorRuntime
}

func newOperatorRuntime() (web.Runtime, error) {
	cfg, err := config.Load()
	if err != nil {
		return nil, err
	}
	if cfg.DatabaseURL == "" || cfg.LedgerKeyringFile == "" || cfg.ServerSigningKeyFile == "" {
		return nil, errors.New("CRAB_DATABASE_URL, CRAB_LEDGER_KEYRING_FILE and CRAB_SERVER_SIGNING_KEY_FILE are required for operator mode")
	}
	keys, err := loadKeyring(cfg.LedgerKeyringFile)
	if err != nil {
		return nil, err
	}
	signer, err := loadOperatorSigner(cfg.ServerSigningKeyFile, keys)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	pool, err := postgres.OpenPool(ctx, cfg.DatabaseURL)
	if err != nil {
		return nil, err
	}
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		pool.Close()
		return nil, err
	}
	if kernel.IntegrityFailed() {
		pool.Close()
		return nil, postgres.ErrIntegrityFailed
	}
	security, err := postgres.NewSecurityStore(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	access, err := postgres.NewAccessStore(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	incidents, err := postgres.NewIncidentStore(pool)
	if err != nil {
		pool.Close()
		return nil, err
	}
	incidentRuntime, err := postgres.NewIncidentRuntime(kernel, signer, incidents)
	if err != nil {
		pool.Close()
		return nil, err
	}
	r := &operatorRuntime{kernel: kernel, signer: signer, security: security, access: access, incidents: incidents}
	r.local = authn.Local{Store: security}
	r.sessions = authn.Sessions{Store: security, TTL: operatorSessionTTL}
	r.incident = app.IncidentService{Backend: incidentRuntime}
	return r, nil
}

func loadOperatorSigner(path string, keys ledger.Keyring) (postgres.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return postgres.Signer{}, fmt.Errorf("read server signing key: %w", err)
	}
	var file signingKeyFile
	if err := json.Unmarshal(b, &file); err != nil {
		return postgres.Signer{}, fmt.Errorf("parse server signing key: %w", err)
	}
	file.KeyID = strings.TrimSpace(file.KeyID)
	if file.KeyID == "" || file.PrivateKey == "" {
		return postgres.Signer{}, errors.New("server signing key requires key_id and private_key")
	}
	privateKey, err := base64.StdEncoding.DecodeString(file.PrivateKey)
	if err != nil || len(privateKey) != ed25519.PrivateKeySize {
		return postgres.Signer{}, errors.New("server signing private key is invalid")
	}
	encodedPublic, ok := keys[file.KeyID]
	if !ok {
		return postgres.Signer{}, errors.New("server signing key id is absent from ledger keyring")
	}
	publicKey, err := base64.StdEncoding.DecodeString(encodedPublic)
	if err != nil || len(publicKey) != ed25519.PublicKeySize {
		return postgres.Signer{}, errors.New("ledger keyring server public key is invalid")
	}
	derived := ed25519.PrivateKey(privateKey).Public().(ed25519.PublicKey)
	if !bytes.Equal(publicKey, derived) {
		return postgres.Signer{}, errors.New("server signing private key does not match ledger keyring")
	}
	return postgres.Signer{KeyID: file.KeyID, PrivateKey: ed25519.PrivateKey(privateKey)}, nil
}

func (r *operatorRuntime) Bootstrapped(ctx context.Context) (bool, error) {
	return r.access.Bootstrapped(ctx)
}

func (r *operatorRuntime) Bootstrap(ctx context.Context, input web.BootstrapInput) error {
	ready, err := r.Bootstrapped(ctx)
	if err != nil {
		return err
	}
	if ready {
		return errors.New("bootstrap already complete")
	}
	if input.PreparedBy == input.ConfirmedBy {
		return errors.New("bootstrap requires two distinct humans")
	}
	prepared, err := authn.NewPasswordRecord(input.PreparedBy, input.PreparedPassword, authn.MinPBKDF2Iterations)
	if err != nil {
		return err
	}
	confirmed, err := authn.NewPasswordRecord(input.ConfirmedBy, input.ConfirmedPassword, authn.MinPBKDF2Iterations)
	if err != nil {
		return err
	}
	if err := r.security.PutPasswordRecord(ctx, prepared); err != nil {
		return err
	}
	if err := r.security.PutPasswordRecord(ctx, confirmed); err != nil {
		return err
	}
	plan := ceremony.BootstrapPlan{
		PreparedBy:  input.PreparedBy,
		ConfirmedBy: input.ConfirmedBy,
		Identities: []ceremony.BootstrapIdentity{
			{HumanID: input.PreparedBy, Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
			{HumanID: input.ConfirmedBy, Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
		},
		BreakGlassKeyholderIDs: []string{input.PreparedBy, input.ConfirmedBy},
	}
	records, err := ceremony.BootstrapRecords(plan, time.Now().UTC())
	if err != nil {
		return err
	}
	_, err = r.kernel.AppendBatch(ctx, records, r.signer, postgres.AccessProjection{})
	return err
}

func (r *operatorRuntime) Authenticate(ctx context.Context, humanID, password string) (string, error) {
	ok, err := r.local.Authenticate(ctx, humanID, password)
	if err != nil {
		return "", err
	}
	if !ok {
		return "", errors.New("authentication failed")
	}
	token, _, err := r.sessions.Create(ctx, humanID, "local-password", false)
	return token, err
}

func (r *operatorRuntime) Logout(ctx context.Context, token string) error {
	return r.sessions.Revoke(ctx, token)
}

func (r *operatorRuntime) Principal(ctx context.Context, token string) (authz.Principal, bool, error) {
	session, ok, err := r.sessions.Resolve(ctx, token)
	if err != nil || !ok {
		return authz.Principal{}, ok, err
	}
	principal, err := r.access.Principal(ctx, session.HumanID, true)
	if err != nil {
		return authz.Principal{}, false, err
	}
	return principal, true, nil
}

func (r *operatorRuntime) ListIncidents(ctx context.Context, principal authz.Principal) ([]web.IncidentLink, error) {
	ids := make([]string, 0, len(principal.Incidents))
	for incidentID := range principal.Incidents {
		ids = append(ids, incidentID)
	}
	sort.Strings(ids)
	out := make([]web.IncidentLink, 0, len(ids))
	for _, incidentID := range ids {
		picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
		if err != nil {
			return nil, err
		}
		if found {
			out = append(out, web.IncidentLink{IncidentID: picture.Incident.IncidentID, Mode: picture.Incident.Mode, Title: picture.Incident.Title, Severity: picture.Incident.Severity})
		}
	}
	return out, nil
}

func (r *operatorRuntime) PendingDeclarations(ctx context.Context, principal authz.Principal) ([]web.PendingDeclaration, error) {
	if !principal.Authenticated || principal.Role != authz.RoleMajor {
		return nil, nil
	}
	values, err := r.incidents.PendingDeclarations(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]web.PendingDeclaration, 0, len(values))
	for _, value := range values {
		out = append(out, web.PendingDeclaration{IncidentID: value.IncidentID, RecordID: value.RecordID, Version: value.Version, Maker: value.Maker, Title: value.Declaration.Title, Summary: value.Declaration.Summary, Severity: value.Declaration.Severity, Mode: value.Mode})
	}
	return out, nil
}

func (r *operatorRuntime) DeclareIncident(ctx context.Context, principal authz.Principal, incidentID, mode, title, summary, severity string) error {
	scope := domain.Scope{IncidentID: incidentID, Mode: mode, Compartment: "GENERAL"}
	_, err := r.incident.ProposeIncidentDeclaration(ctx, principal, scope, newOperatorID("DECL"), domain.IncidentDeclaration{Title: title, Summary: summary, Severity: severity}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) Picture(ctx context.Context, principal authz.Principal, incidentID, compartment string) (domain.OperatingPicture, bool, error) {
	return r.incident.CommonOperatingPicture(ctx, principal, incidentID, compartment)
}

func (r *operatorRuntime) RecordFact(ctx context.Context, principal authz.Principal, incidentID, statement, source string) error {
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil || !found {
		if err != nil {
			return err
		}
		return errors.New("incident not found")
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.incident.RecordFact(ctx, principal, scope, newOperatorID("FACT"), domain.Fact{Statement: statement, Source: source}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeAction(ctx context.Context, principal authz.Principal, incidentID string, action domain.Action) error {
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil || !found {
		if err != nil {
			return err
		}
		return errors.New("incident not found")
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}
	_, err = r.incident.ProposeAction(ctx, principal, scope, newOperatorID("ACT"), action, time.Now().UTC())
	return err
}

func (r *operatorRuntime) Decide(ctx context.Context, principal authz.Principal, incidentID, recordType, recordID, version, outcome string) error {
	_, err := r.incident.Decide(ctx, principal, incidentID, recordType, recordID, version, outcome, time.Now().UTC())
	return err
}

func newOperatorID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}
