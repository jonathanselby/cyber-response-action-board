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

	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/artifacts"
	"github.com/jonathanselby/cyber-response-action-board/internal/adapters/notifier"
	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/buildinfo"
	"github.com/jonathanselby/cyber-response-action-board/internal/config"
	"github.com/jonathanselby/cyber-response-action-board/internal/data/postgres"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	pdfpresentation "github.com/jonathanselby/cyber-response-action-board/internal/presentation/pdf"
	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/ceremony"
)

const operatorSessionTTL = 8 * time.Hour

type operatorRuntime struct {
	kernel          *postgres.Kernel
	signer          postgres.Signer
	security        *postgres.SecurityStore
	access          *postgres.AccessStore
	incidents       *postgres.IncidentStore
	incident        app.IncidentService
	accessService   app.AccessService
	evidence        app.EvidenceService
	communication   app.CommunicationService
	recovery        app.RecoveryService
	retrieval       app.RetrievalService
	print           app.PrintService
	pageSizeDefault string
	local           authn.Local
	sessions        authn.Sessions
}

type signingKeyFile struct {
	KeyID      string `json:"key_id"`
	PrivateKey string `json:"private_key"`
}

func init() { web.RuntimeFactory = newOperatorRuntime }

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
	fail := func(err error) (web.Runtime, error) {
		pool.Close()
		return nil, err
	}
	kernel, err := postgres.OpenKernel(ctx, pool, keys)
	if err != nil {
		return fail(err)
	}
	if kernel.IntegrityFailed() {
		return fail(postgres.ErrIntegrityFailed)
	}
	security, err := postgres.NewSecurityStore(pool)
	if err != nil {
		return fail(err)
	}
	accessStore, err := postgres.NewAccessStore(pool)
	if err != nil {
		return fail(err)
	}
	incidents, err := postgres.NewIncidentStore(pool)
	if err != nil {
		return fail(err)
	}
	incidentBackend, err := postgres.NewIncidentRuntime(kernel, signer, incidents)
	if err != nil {
		return fail(err)
	}
	accessBackend, err := postgres.NewAccessRuntime(kernel, signer, accessStore)
	if err != nil {
		return fail(err)
	}
	blobStore, err := postgres.NewBlobStore(pool, cfg.EvidenceMaxBytes)
	if err != nil {
		return fail(err)
	}
	evidenceStore, err := postgres.NewEvidenceStore(pool)
	if err != nil {
		return fail(err)
	}
	evidenceBackend, err := postgres.NewEvidenceRuntime(kernel, signer, blobStore, evidenceStore)
	if err != nil {
		return fail(err)
	}
	communicationStore, err := postgres.NewCommunicationStore(pool)
	if err != nil {
		return fail(err)
	}
	communicationBackend, err := postgres.NewCommunicationRuntime(kernel, signer, communicationStore)
	if err != nil {
		return fail(err)
	}
	recoveryStore, err := postgres.NewRecoveryStore(pool)
	if err != nil {
		return fail(err)
	}
	recoveryBackend, err := postgres.NewRecoveryRuntime(kernel, signer, recoveryStore)
	if err != nil {
		return fail(err)
	}
	channels, err := loadNotificationDeployment(cfg)
	if err != nil {
		return fail(err)
	}

	r := &operatorRuntime{kernel: kernel, signer: signer, security: security, access: accessStore, incidents: incidents}
	r.local = authn.Local{Store: security}
	r.sessions = authn.Sessions{Store: security, TTL: operatorSessionTTL}
	r.incident = app.IncidentService{Backend: incidentBackend}
	r.accessService = app.AccessService{Backend: accessBackend}
	r.evidence = app.EvidenceService{Backend: evidenceBackend}
	r.communication = app.CommunicationService{Backend: communicationBackend, Incident: r.incident, Notifier: notifier.Sender{}, Channels: channels}
	r.recovery = app.RecoveryService{Backend: recoveryBackend}
	if corpusStore, err := postgres.NewCorpusStore(pool); err == nil {
		if corpusBackend, err := postgres.NewCorpusRuntime(kernel, signer, blobStore, corpusStore); err == nil {
			r.retrieval = app.RetrievalService{Backend: corpusBackend}
		}
	}
	r.pageSizeDefault = cfg.PageSizeDefault
	if strings.TrimSpace(cfg.ExportDir) != "" {
		printSource, err := postgres.NewPrintStore(pool)
		if err != nil {
			return fail(err)
		}
		artifactStore, err := artifacts.New(cfg.ExportDir, cfg.SecondaryExportDir)
		if err != nil {
			return fail(err)
		}
		r.print = app.PrintService{Incident: r.incident, Evidence: printEvidenceReader{store: evidenceStore}, Source: printSource, Renderer: pdfpresentation.Renderer{}, Artifacts: artifactStore, ConfigurationVersion: buildinfo.Version, TimeStatus: func() string { return kernel.Clock().Read().Status }}
	}
	return r, nil
}

func loadOperatorSigner(path string, keys ledger.Keyring) (postgres.Signer, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return postgres.Signer{}, fmt.Errorf("read server signing key: %w", err)
	}
	var f signingKeyFile
	if err := json.Unmarshal(b, &f); err != nil {
		return postgres.Signer{}, fmt.Errorf("parse server signing key: %w", err)
	}
	f.KeyID = strings.TrimSpace(f.KeyID)
	raw, err := base64.StdEncoding.DecodeString(f.PrivateKey)
	if f.KeyID == "" || err != nil || len(raw) != ed25519.PrivateKeySize {
		return postgres.Signer{}, errors.New("server signing key is invalid")
	}
	encoded, ok := keys[f.KeyID]
	if !ok {
		return postgres.Signer{}, errors.New("server signing key id is absent from ledger keyring")
	}
	want, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil || len(want) != ed25519.PublicKeySize {
		return postgres.Signer{}, errors.New("ledger keyring server public key is invalid")
	}
	private := ed25519.PrivateKey(raw)
	if !bytes.Equal(want, private.Public().(ed25519.PublicKey)) {
		return postgres.Signer{}, errors.New("server signing private key does not match ledger keyring")
	}
	return postgres.Signer{KeyID: f.KeyID, PrivateKey: private}, nil
}

func (r *operatorRuntime) Bootstrapped(ctx context.Context) (bool, error) {
	return r.access.Bootstrapped(ctx)
}

func (r *operatorRuntime) Bootstrap(ctx context.Context, input web.BootstrapInput) error {
	if input.PreparedBy == input.ConfirmedBy {
		return errors.New("bootstrap requires two distinct humans")
	}
	for _, password := range []string{input.PreparedPassword, input.ConfirmedPassword} {
		if err := authn.ValidateNewPassword(password); err != nil {
			return err
		}
	}
	ready, err := r.Bootstrapped(ctx)
	if err != nil {
		return err
	}
	if ready {
		return errors.New("bootstrap already complete")
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
	for i := range records {
		if records[i].MakerSignature.KeyID == "" && records[i].MakerSignature.Value == "" {
			records[i].MakerSignature.MissingReason = "maker authenticated in two-person bootstrap ceremony"
		}
		if records[i].WriteSemantic == "governed" && records[i].Status != "proposed" && records[i].ConfirmerSignature.KeyID == "" && records[i].ConfirmerSignature.Value == "" {
			records[i].ConfirmerSignature.MissingReason = "confirmer authenticated in two-person bootstrap ceremony"
		}
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

func (r *operatorRuntime) principal(ctx context.Context, token string) (authz.Principal, bool, error) {
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

func (r *operatorRuntime) Home(ctx context.Context, token string) (web.HomeView, bool, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil || !ok {
		return web.HomeView{}, ok, err
	}
	ids := make([]string, 0, len(principal.Incidents))
	for id := range principal.Incidents {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	home := web.HomeView{HumanID: principal.HumanID, Role: string(principal.Role), CanDeclare: principal.Role == authz.RoleMajor}
	for _, id := range ids {
		picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, id, "GENERAL")
		if err != nil {
			return web.HomeView{}, false, err
		}
		if found {
			home.Incidents = append(home.Incidents, web.IncidentLink{IncidentID: id, Mode: picture.Incident.Mode, Title: picture.Incident.Title, Severity: picture.Incident.Severity})
		}
	}
	if principal.Role == authz.RoleMajor {
		values, err := r.incidents.PendingDeclarations(ctx)
		if err != nil {
			return web.HomeView{}, false, err
		}
		for _, v := range values {
			home.PendingDeclarations = append(home.PendingDeclarations, web.PendingDeclaration{IncidentID: v.IncidentID, RecordID: v.RecordID, Version: v.Version, Maker: v.Maker, Title: v.Declaration.Title, Summary: v.Declaration.Summary, Severity: v.Declaration.Severity, Mode: v.Mode})
		}
	}
	return home, true, nil
}

func (r *operatorRuntime) Incident(ctx context.Context, token, incidentID string) (web.IncidentView, bool, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil || !ok {
		return web.IncidentView{}, false, err
	}
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil || !found {
		return web.IncidentView{}, found, err
	}
	out := web.IncidentView{IncidentID: picture.Incident.IncidentID, Mode: picture.Incident.Mode, Title: picture.Incident.Title, Summary: picture.Incident.Summary, Severity: picture.Incident.Severity}
	for _, v := range picture.Facts {
		out.Facts = append(out.Facts, web.FactView{Statement: v.Fact.Statement, Source: v.Fact.Source})
	}
	for _, v := range picture.Actions {
		out.Actions = append(out.Actions, web.ActionView{RecordID: v.RecordID, Title: v.Action.Title, Owner: v.Action.OwnerHumanID, Status: v.Action.Status, Priority: v.Action.Priority})
	}
	for _, v := range picture.Pending {
		out.Pending = append(out.Pending, web.PendingView{RecordType: v.RecordType, RecordID: v.RecordID, Version: v.Version, Maker: v.Maker})
	}
	for _, v := range picture.Timeline {
		out.Timeline = append(out.Timeline, web.TimelineView{Summary: v.Summary, RecordedAt: v.RecordedAt.UTC().Format(time.RFC3339), Governance: v.Governance})
	}
	return out, true, nil
}

func (r *operatorRuntime) DeclareIncident(ctx context.Context, token, incidentID, mode, title, summary, severity string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	scope := domain.Scope{IncidentID: incidentID, Mode: mode, Compartment: "GENERAL"}
	_, err = r.incident.ProposeIncidentDeclaration(ctx, principal, scope, newOperatorID("DECL"), domain.IncidentDeclaration{Title: title, Summary: summary, Severity: severity}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) RecordFact(ctx context.Context, token, incidentID, statement, source string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	_, err = r.incident.RecordFact(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, newOperatorID("FACT"), domain.Fact{Statement: statement, Source: source}, time.Now().UTC())
	return err
}

func (r *operatorRuntime) ProposeAction(ctx context.Context, token, incidentID, title, owner, priority, sourceRef string) error {
	principal, picture, err := r.scope(ctx, token, incidentID)
	if err != nil {
		return err
	}
	action := domain.Action{Title: title, OwnerHumanID: owner, Status: domain.ActionStatusNotStarted, Priority: priority, SourceRef: sourceRef}
	_, err = r.incident.ProposeAction(ctx, principal, domain.Scope{IncidentID: incidentID, Mode: picture.Incident.Mode, Compartment: "GENERAL"}, newOperatorID("ACT"), action, time.Now().UTC())
	return err
}

func (r *operatorRuntime) Decide(ctx context.Context, token, incidentID, recordType, recordID, version, outcome string) error {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return err
	}
	if !ok {
		return app.ErrAuthorisationDenied
	}
	_, err = r.incident.Decide(ctx, principal, incidentID, recordType, recordID, version, outcome, time.Now().UTC())
	return err
}

func (r *operatorRuntime) scope(ctx context.Context, token, incidentID string) (authz.Principal, domain.OperatingPicture, error) {
	principal, ok, err := r.principal(ctx, token)
	if err != nil {
		return authz.Principal{}, domain.OperatingPicture{}, err
	}
	if !ok {
		return authz.Principal{}, domain.OperatingPicture{}, app.ErrAuthorisationDenied
	}
	picture, found, err := r.incident.CommonOperatingPicture(ctx, principal, incidentID, "GENERAL")
	if err != nil {
		return authz.Principal{}, domain.OperatingPicture{}, err
	}
	if !found {
		return authz.Principal{}, domain.OperatingPicture{}, app.ErrAuthorisationDenied
	}
	return principal, picture, nil
}

func newOperatorID(prefix string) string {
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return fmt.Sprintf("%s-%d", prefix, time.Now().UTC().UnixNano())
	}
	return prefix + "-" + hex.EncodeToString(raw[:])
}
