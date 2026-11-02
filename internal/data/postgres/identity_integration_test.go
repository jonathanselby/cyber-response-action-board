package postgres

import (
	"context"
	"crypto/ed25519"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha1"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/app"
	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/ceremony"
)

// identityFixture is a bootstrapped CRAB with three Custodians (alice, bob,
// carol) built through the real bootstrap ceremony and identity service.
type identityFixture struct {
	t          *testing.T
	ctx        context.Context
	db         *testDB
	kernel     *Kernel
	signer     Signer
	store      *AccessStore
	service    app.AccessService
	creds      CredentialStore
	auth       app.Authentication
	principals app.Principals
	sessions   authn.Sessions
	now        time.Time
	seq        int
}

func newIdentityFixture(t *testing.T) *identityFixture {
	t.Helper()
	db := setupTestDB(t)
	ctx := context.Background()
	serverPub, serverPriv, _ := ed25519.GenerateKey(rand.Reader)
	keyring := ledger.Keyring{"server": base64.StdEncoding.EncodeToString(serverPub)}
	kernel, err := OpenKernel(ctx, db.runtime, keyring)
	if err != nil {
		t.Fatal(err)
	}
	signer := Signer{KeyID: "server", PrivateKey: serverPriv}
	f := &identityFixture{t: t, ctx: ctx, db: db, kernel: kernel, signer: signer, now: time.Date(2026, 11, 2, 2, 17, 0, 0, time.UTC)}
	records, err := ceremony.BootstrapRecords(ceremony.BootstrapPlan{
		PreparedBy: "alice", ConfirmedBy: "bob",
		Identities: []ceremony.BootstrapIdentity{
			{HumanID: "alice", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
			{HumanID: "bob", Custodian: true, MajorEligible: true, LocalAuthConfigured: true},
		},
		BreakGlassKeyholderIDs: []string{"alice", "bob"},
	}, f.now)
	if err != nil {
		t.Fatal(err)
	}
	for i := range records {
		records[i].MakerSignature = ledger.Signature{MissingReason: "test bootstrap"}
		records[i].ConfirmerSignature = ledger.Signature{MissingReason: "test bootstrap"}
	}
	if _, err := kernel.AppendBatch(ctx, records, signer, AccessProjection{}); err != nil {
		t.Fatal(err)
	}
	f.store, err = NewAccessStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	runtime, err := NewAccessRuntime(kernel, signer, f.store)
	if err != nil {
		t.Fatal(err)
	}
	security, err := NewSecurityStore(db.runtime)
	if err != nil {
		t.Fatal(err)
	}
	wrap := make([]byte, authn.CredentialWrapKeyBytes)
	_, _ = rand.Read(wrap)
	f.service = app.AccessService{Backend: runtime}
	f.creds = CredentialStore{Security: security, WrapKey: wrap}
	clock := func() time.Time { return f.now }
	f.sessions = authn.Sessions{Store: security, TTL: 8 * time.Hour, Now: clock}
	f.principals = app.Principals{Sessions: f.sessions, Source: f.store, Now: clock}
	f.auth = app.Authentication{Credentials: f.creds, Sessions: f.sessions, Identities: f.store, Issuer: "CRAB"}
	for _, id := range []string{"alice", "bob"} {
		f.setPassword(id, "Bafflewick-"+id+"-26!")
	}
	// A third Custodian, created through the identity service itself.
	f.createPerson("alice", "bob", access.IdentityChange{HumanID: "carol", DisplayName: "Carol Custodian", Status: access.IdentityActive}, authz.RoleMajor)
	f.decide("alice", "bob", f.propose(f.service.ProposeRoleEligibility(f.ctx, f.principal("alice"), f.id("CUST"), "carol", authz.RoleMajor, true, f.now)))
	custodian, err := domain.ProposeSystemGoverned(access.RecordCustodianDesignation, f.id("CUSTODIAN"), "alice", f.now, map[string]any{"human_id": "carol", "active": true})
	if err != nil {
		t.Fatal(err)
	}
	if err := runtime.AppendAccess(ctx, custodian); err != nil {
		t.Fatal(err)
	}
	f.decide("alice", "bob", custodian)
	f.setPassword("carol", "Bafflewick-carol-26!")
	return f
}

func (f *identityFixture) id(prefix string) string {
	f.seq++
	return fmt.Sprintf("%s-%03d", prefix, f.seq)
}

func (f *identityFixture) setPassword(humanID, password string) {
	f.t.Helper()
	record, err := authn.NewPasswordRecord(humanID, password, authn.MinPBKDF2Iterations)
	if err != nil {
		f.t.Fatal(err)
	}
	if err := f.creds.PutPassword(f.ctx, record); err != nil {
		f.t.Fatal(err)
	}
}

// principal loads a principal directly (as if already signed in), at f.now.
func (f *identityFixture) principal(humanID string) authz.Principal {
	f.t.Helper()
	p, err := f.store.PrincipalAt(f.ctx, humanID, true, f.now)
	if err != nil {
		f.t.Fatal(err)
	}
	return p
}

func (f *identityFixture) propose(record ledger.Record, err error) ledger.Record {
	f.t.Helper()
	if err != nil {
		f.t.Fatal(err)
	}
	return record
}

func (f *identityFixture) decideErr(confirmer string, proposal ledger.Record, outcome string) error {
	version, err := domain.ProposalVersion(proposal)
	if err != nil {
		f.t.Fatal(err)
	}
	_, err = f.service.Decide(f.ctx, f.principal(confirmer), "", proposal.RecordType, proposal.RecordID, version, outcome, f.now)
	return err
}

func (f *identityFixture) decide(_ string, confirmer string, proposal ledger.Record) {
	f.t.Helper()
	if err := f.decideErr(confirmer, proposal, "confirmed"); err != nil {
		f.t.Fatalf("confirm %s by %s: %v", proposal.RecordType, confirmer, err)
	}
}

func (f *identityFixture) createPerson(maker, confirmer string, change access.IdentityChange, role authz.Role) {
	f.t.Helper()
	f.decide(maker, confirmer, f.propose(f.service.ProposeIdentity(f.ctx, f.principal(maker), f.id("IDENTITY"), change, f.now)))
}

// reset issues an enrolment code through the governed two-Custodian path.
func (f *identityFixture) reset(maker, confirmer, humanID string) string {
	f.t.Helper()
	proposal := f.propose(f.service.ProposeCredentialReset(f.ctx, f.principal(maker), f.id("RESET"), humanID, "new responder", f.now))
	version, _ := domain.ProposalVersion(proposal)
	issued, err := f.service.ConfirmCredentialReset(f.ctx, f.principal(confirmer), f.creds, proposal.RecordID, version, f.now)
	if err != nil {
		f.t.Fatalf("confirm reset: %v", err)
	}
	if issued.HumanID != humanID {
		f.t.Fatalf("issued code for %q, want %q", issued.HumanID, humanID)
	}
	return issued.Code
}

func (f *identityFixture) signIn(humanID, secret, totp string) string {
	f.t.Helper()
	token, err := f.auth.Authenticate(f.ctx, humanID, secret, totp, f.now)
	if err != nil {
		f.t.Fatalf("sign in %s: %v", humanID, err)
	}
	return token
}

func (f *identityFixture) tokenWorks(token string) bool {
	f.t.Helper()
	_, _, ok, err := f.principals.ForToken(f.ctx, token)
	if err != nil {
		f.t.Fatal(err)
	}
	return ok
}

func totpNow(t *testing.T, secret string, now time.Time) string {
	t.Helper()
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(secret)
	if err != nil {
		t.Fatal(err)
	}
	var msg [8]byte
	binary.BigEndian.PutUint64(msg[:], uint64(now.Unix()/30))
	mac := hmac.New(sha1.New, raw)
	mac.Write(msg[:])
	sum := mac.Sum(nil)
	off := sum[len(sum)-1] & 0x0f
	code := (binary.BigEndian.Uint32(sum[off:off+4]) & 0x7fffffff) % 1000000
	return fmt.Sprintf("%06d", code)
}

// T-214: identity changes need a Custodian maker and a different Custodian
// confirmer; disabled is final; every change is a ledger version.
func TestIdentityLifecycleIsDualControlledAndDisabledIsFinal(t *testing.T) {
	f := newIdentityFixture(t)
	jane := access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Organisation: "Forensic Co", Status: access.IdentityActive}

	// A person without Custodian authority cannot propose.
	f.createPerson("alice", "bob", access.IdentityChange{HumanID: "minnie", DisplayName: "Minnie Minor", Status: access.IdentityActive}, authz.RoleMinor)
	if _, err := f.service.ProposeIdentity(f.ctx, f.principal("minnie"), f.id("X"), jane, f.now); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("non-Custodian proposal err=%v", err)
	}
	proposal := f.propose(f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), jane, f.now))
	if err := f.decideErr("alice", proposal, "confirmed"); err == nil {
		t.Fatal("maker confirmed own identity proposal")
	}
	if err := f.decideErr("minnie", proposal, "confirmed"); err == nil {
		t.Fatal("non-Custodian confirmed identity proposal")
	}
	f.decide("alice", "bob", proposal)
	state, _, _ := f.store.IdentityState(f.ctx, "jane")
	if !state.Found || state.Status != access.IdentityActive || state.Organisation != "Forensic Co" {
		t.Fatalf("identity not projected: %+v", state)
	}

	suspend := jane
	suspend.Status, suspend.Reason = access.IdentitySuspended, "left the site"
	f.decide("bob", "carol", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("bob"), f.id("IDENTITY"), suspend, f.now)))
	f.decide("carol", "alice", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("carol"), f.id("IDENTITY"), jane, f.now)))
	disable := jane
	disable.Status, disable.Reason = access.IdentityDisabled, "engagement ended"
	f.setPassword("jane", "Forensic-Night-26!")
	f.decide("alice", "bob", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), disable, f.now)))
	if _, found, _ := f.creds.GetCredential(f.ctx, "jane"); found {
		t.Fatal("disabled person's credential material was kept")
	}
	if _, err := f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), jane, f.now); !errors.Is(err, access.ErrIdentityDisabled) {
		t.Fatalf("reactivating a disabled identity err=%v", err)
	}

	var versions int
	if err := f.db.runtime.QueryRow(f.ctx, `SELECT count(*) FROM crab.ledger_entries WHERE convert_from(entry_bytes,'UTF8')::jsonb #>> '{record,record_type}' = 'security.identity' AND convert_from(entry_bytes,'UTF8')::jsonb #>> '{record,data}' LIKE '%jane%' AND convert_from(entry_bytes,'UTF8')::jsonb #>> '{record,status}' = 'confirmed'`).Scan(&versions); err != nil {
		t.Fatal(err)
	}
	if versions != 4 {
		t.Fatalf("confirmed identity versions for jane=%d, want 4", versions)
	}

	// Rebuild reproduces the same identity state.
	if _, err := f.kernel.Rebuild(f.ctx, RebuildProjections()...); err != nil {
		t.Fatal(err)
	}
	after, _, _ := f.store.IdentityState(f.ctx, "jane")
	if after.Status != access.IdentityDisabled {
		t.Fatalf("rebuilt identity status=%s", after.Status)
	}
}

// T-215: role eligibility after setup needs a Custodian maker, a different
// confirmer and an active identity record.
func TestRoleEligibilityAfterSetupNeedsIdentityAndDualControl(t *testing.T) {
	f := newIdentityFixture(t)
	if _, err := f.service.ProposeRoleEligibility(f.ctx, f.principal("alice"), f.id("ROLE"), "ghost", authz.RoleMinor, true, f.now); !errors.Is(err, access.ErrIdentityRecordMissing) {
		t.Fatalf("eligibility for a person with no identity err=%v", err)
	}
	f.createPerson("alice", "bob", access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Status: access.IdentityActive}, authz.RoleMinor)
	proposal := f.propose(f.service.ProposeRoleEligibility(f.ctx, f.principal("alice"), f.id("ROLE"), "jane", authz.RoleMinor, true, f.now))
	if err := f.decideErr("alice", proposal, "confirmed"); err == nil {
		t.Fatal("maker confirmed own eligibility proposal")
	}
	f.decide("alice", "bob", proposal)
	if p := f.principal("jane"); p.Role != authz.RoleMinor {
		t.Fatalf("jane role=%q", p.Role)
	}
}

// T-216, T-217: enrolment codes are dual-controlled, shown only to the
// confirmer, stored hashed, expire, force a compliant password, and a reset
// ends every session.
func TestEnrolmentCodeLifecycle(t *testing.T) {
	f := newIdentityFixture(t)
	f.createPerson("alice", "bob", access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Status: access.IdentityActive}, authz.RoleMinor)
	f.decide("alice", "bob", f.propose(f.service.ProposeRoleEligibility(f.ctx, f.principal("alice"), f.id("ROLE"), "jane", authz.RoleMinor, true, f.now)))

	proposal := f.propose(f.service.ProposeCredentialReset(f.ctx, f.principal("alice"), f.id("RESET"), "jane", "new responder", f.now))
	version, _ := domain.ProposalVersion(proposal)
	if err := f.decideErr("bob", proposal, "confirmed"); !errors.Is(err, app.ErrUseCredentialResetPath) {
		t.Fatalf("generic confirm of a reset err=%v", err)
	}
	if _, err := f.service.ConfirmCredentialReset(f.ctx, f.principal("alice"), f.creds, proposal.RecordID, version, f.now); err == nil {
		t.Fatal("maker confirmed own credential reset")
	}
	issued, err := f.service.ConfirmCredentialReset(f.ctx, f.principal("bob"), f.creds, proposal.RecordID, version, f.now)
	if err != nil {
		t.Fatal(err)
	}
	code, expires := issued.Code, issued.ExpiresAt
	if !expires.Equal(f.now.Add(app.EnrolmentCodeLifetime)) || len(strings.ReplaceAll(code, "-", "")) != 20 {
		t.Fatalf("code=%q expires=%s", code, expires)
	}

	// Nothing durable holds the code in clear.
	var leaked int
	if err := f.db.runtime.QueryRow(f.ctx, `SELECT (SELECT count(*) FROM crab.ledger_entries WHERE convert_from(entry_bytes,'UTF8') LIKE '%'||$1||'%') + (SELECT count(*) FROM crab.local_credentials WHERE password_hash LIKE '%'||$1||'%' OR salt LIKE '%'||$1||'%')`, strings.ReplaceAll(code, "-", "")).Scan(&leaked); err != nil {
		t.Fatal(err)
	}
	if leaked != 0 {
		t.Fatal("enrolment code stored in clear")
	}

	// The code alone creates no session; a weak password is refused.
	if _, err := f.auth.Authenticate(f.ctx, "jane", code, "", f.now); !errors.Is(err, app.ErrEnrolmentRequired) {
		t.Fatalf("sign-in with code err=%v", err)
	}
	if _, err := f.auth.CompleteEnrolment(f.ctx, "jane", strings.ToLower(code), "short", "", f.now); err == nil {
		t.Fatal("weak password accepted at enrolment")
	}
	token, err := f.auth.CompleteEnrolment(f.ctx, "jane", strings.ToLower(code), "Forensic-Night-26!", "", f.now)
	if err != nil || !f.tokenWorks(token) {
		t.Fatalf("complete enrolment err=%v", err)
	}
	if _, err := f.auth.Authenticate(f.ctx, "jane", code, "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("used code still accepted err=%v", err)
	}
	second := f.signIn("jane", "Forensic-Night-26!", "")

	// A reset ends every session and the old password.
	newCode := f.reset("carol", "alice", "jane")
	if f.tokenWorks(token) || f.tokenWorks(second) {
		t.Fatal("session survived credential reset")
	}
	if _, err := f.auth.Authenticate(f.ctx, "jane", "Forensic-Night-26!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("old password survived reset err=%v", err)
	}
	// An expired code is refused.
	f.now = f.now.Add(app.EnrolmentCodeLifetime + time.Minute)
	if _, err := f.auth.CompleteEnrolment(f.ctx, "jane", newCode, "Forensic-Night-27!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("expired code err=%v", err)
	}
}

// T-218: suspension, disablement and account expiry refuse sign-in and the
// next request of every existing session.
func TestAccountStateEndsLiveSessions(t *testing.T) {
	f := newIdentityFixture(t)
	expiry := f.now.Add(6 * time.Hour)
	jane := access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Status: access.IdentityActive, AccountExpiresAt: &expiry}
	f.createPerson("alice", "bob", jane, authz.RoleMinor)
	f.setPassword("jane", "Forensic-Night-26!")
	browser, api := f.signIn("jane", "Forensic-Night-26!", ""), f.signIn("jane", "Forensic-Night-26!", "")

	suspend := jane
	suspend.Status, suspend.Reason = access.IdentitySuspended, "laptop lost"
	f.decide("alice", "bob", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), suspend, f.now)))
	if f.tokenWorks(browser) || f.tokenWorks(api) {
		t.Fatal("session survived suspension")
	}
	var remaining int
	_ = f.db.runtime.QueryRow(f.ctx, `SELECT count(*) FROM crab.sessions WHERE human_id='jane'`).Scan(&remaining)
	if remaining != 0 {
		t.Fatalf("suspended person still holds %d sessions", remaining)
	}
	if _, err := f.auth.Authenticate(f.ctx, "jane", "Forensic-Night-26!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("suspended sign-in err=%v", err)
	}

	// Reactivate; then the account expiry ends the session with no timer.
	f.decide("bob", "carol", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("bob"), f.id("IDENTITY"), jane, f.now)))
	token := f.signIn("jane", "Forensic-Night-26!", "")
	f.now = expiry.Add(time.Second)
	if f.tokenWorks(token) {
		t.Fatal("session survived account expiry")
	}
	if _, err := f.auth.Authenticate(f.ctx, "jane", "Forensic-Night-26!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("expired account sign-in err=%v", err)
	}
}

// T-219: incident membership expiry is evaluated against the governed clock
// supplied to the principal rule, not the host wall clock.
func TestMembershipExpiryFollowsGovernedClock(t *testing.T) {
	f := newIdentityFixture(t)
	f.now = time.Date(2031, 1, 1, 0, 0, 0, 0, time.UTC) // far from the real wall clock
	scope := domain.Scope{IncidentID: "INC-004", Mode: "live", Compartment: "GENERAL"}
	_, err := f.db.runtime.Exec(f.ctx, `INSERT INTO crab.incident_memberships(human_id,incident_id,effective_at,expires_at,reason) VALUES('alice',$1,$2,$3,'fixture')`, scope.IncidentID, f.now.Add(-time.Hour), f.now.Add(time.Hour))
	if err != nil {
		t.Fatal(err)
	}
	token := f.signIn("alice", "Bafflewick-alice-26!", "")
	p, _, ok, _ := f.principals.ForToken(f.ctx, token)
	if !ok || !p.Incidents["INC-004"] {
		t.Fatal("membership not active at the governed time")
	}
	f.now = f.now.Add(2 * time.Hour)
	if p, _, ok, _ := f.principals.ForToken(f.ctx, token); !ok || p.Incidents["INC-004"] {
		t.Fatal("expired membership still granted")
	}
}

// T-220: expired sessions are swept on sign-in, bounded per sign-in.
func TestExpiredSessionsAreSwept(t *testing.T) {
	f := newIdentityFixture(t)
	for i := 0; i < 150; i++ {
		var digest [32]byte
		_, _ = rand.Read(digest[:])
		if _, err := f.db.runtime.Exec(f.ctx, `INSERT INTO crab.sessions(token_digest,human_id,auth_method,created_at,expires_at) VALUES($1,'alice','local-password',$2,$3)`, digest[:], f.now.Add(-10*time.Hour), f.now.Add(-time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	count := func() int {
		var n int
		_ = f.db.runtime.QueryRow(f.ctx, `SELECT count(*) FROM crab.sessions WHERE expires_at <= $1`, f.now).Scan(&n)
		return n
	}
	f.signIn("alice", "Bafflewick-alice-26!", "")
	if n := count(); n != 50 {
		t.Fatalf("expired sessions after one sign-in=%d, want 50", n)
	}
	f.signIn("bob", "Bafflewick-bob-26!", "")
	if n := count(); n != 0 {
		t.Fatalf("expired sessions after two sign-ins=%d, want 0", n)
	}
}

// T-248: unusable Custodians cannot confirm, and a proposal whose maker has
// become unusable can be rejected but not confirmed.
func TestUnusableCustodiansCannotCompleteGovernedChanges(t *testing.T) {
	f := newIdentityFixture(t)
	pending := f.propose(f.service.ProposeIdentity(f.ctx, f.principal("carol"), f.id("IDENTITY"), access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Status: access.IdentityActive}, f.now))
	carolPrincipal := f.principal("carol")

	suspendCarol := access.IdentityChange{HumanID: "carol", DisplayName: "Carol Custodian", Status: access.IdentitySuspended, Reason: "on leave"}
	f.decide("alice", "bob", f.propose(f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), suspendCarol, f.now)))

	// carol's proposal can no longer be confirmed by anyone, only rejected.
	if err := f.decideErr("alice", pending, "confirmed"); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("confirming an unusable maker's proposal err=%v", err)
	}
	if err := f.decideErr("alice", pending, "rejected"); err != nil {
		t.Fatalf("rejecting an unusable maker's proposal: %v", err)
	}
	// carol herself, still holding an old principal, cannot confirm.
	another := f.propose(f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), access.IdentityChange{HumanID: "kim", DisplayName: "Kim", Status: access.IdentityActive}, f.now))
	version, _ := domain.ProposalVersion(another)
	if _, err := f.service.Decide(f.ctx, carolPrincipal, "", another.RecordType, another.RecordID, version, "confirmed", f.now); !errors.Is(err, app.ErrAuthorisationDenied) {
		t.Fatalf("suspended Custodian confirmed err=%v", err)
	}
	if _, err := f.auth.Authenticate(f.ctx, "carol", "Bafflewick-carol-26!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("suspended Custodian signed in err=%v", err)
	}
}

// T-250: external people need TOTP; internal sign-in is unchanged; a reset
// clears TOTP enrolment.
func TestExternalPeopleNeedTOTP(t *testing.T) {
	f := newIdentityFixture(t)
	expiry := f.now.Add(7 * 24 * time.Hour)
	f.createPerson("alice", "bob", access.IdentityChange{HumanID: "jane", DisplayName: "Jane Doe", Organisation: "Forensic Co", External: true, Status: access.IdentityActive, AccountExpiresAt: &expiry}, authz.RoleMinor)

	// External without expiry, or beyond 30 days, is refused.
	if _, err := f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), access.IdentityChange{HumanID: "ext2", DisplayName: "No Expiry", External: true, Status: access.IdentityActive}, f.now); !errors.Is(err, access.ErrExternalNeedsExpiry) {
		t.Fatalf("external without expiry err=%v", err)
	}
	far := f.now.Add(31 * 24 * time.Hour)
	if _, err := f.service.ProposeIdentity(f.ctx, f.principal("alice"), f.id("IDENTITY"), access.IdentityChange{HumanID: "ext3", DisplayName: "Too Long", External: true, Status: access.IdentityActive, AccountExpiresAt: &far}, f.now); !errors.Is(err, access.ErrAccountExpiryTooLong) {
		t.Fatalf("external beyond 30 days err=%v", err)
	}

	code := f.reset("alice", "bob", "jane")
	enrolment, err := f.auth.BeginEnrolment(f.ctx, "jane", code, f.now)
	if err != nil || !enrolment.NeedsTOTP || !strings.HasPrefix(enrolment.TOTPURI, "otpauth://totp/") {
		t.Fatalf("begin enrolment=%+v err=%v", enrolment, err)
	}
	if _, err := f.auth.CompleteEnrolment(f.ctx, "jane", code, "Forensic-Night-26!", "000000", f.now); err == nil {
		t.Fatal("enrolment completed with a wrong TOTP code")
	}
	if _, err := f.auth.CompleteEnrolment(f.ctx, "jane", code, "Forensic-Night-26!", totpNow(t, enrolment.TOTPSecret, f.now), f.now); err != nil {
		t.Fatal(err)
	}
	if _, err := f.auth.Authenticate(f.ctx, "jane", "Forensic-Night-26!", "", f.now); !errors.Is(err, app.ErrAuthenticationFailed) {
		t.Fatalf("external sign-in without TOTP err=%v", err)
	}
	f.signIn("jane", "Forensic-Night-26!", totpNow(t, enrolment.TOTPSecret, f.now))
	// Internal people are not asked for TOTP.
	f.signIn("alice", "Bafflewick-alice-26!", "")

	// A reset clears TOTP enrolment.
	f.reset("bob", "carol", "jane")
	if _, _, found, _ := f.creds.GetTOTP(f.ctx, "jane"); found {
		t.Fatal("TOTP survived credential reset")
	}
}

// Legacy accounts (eligibility from before identity records) stay usable, but
// eligibility cannot be granted to someone without an identity record.
func TestLegacyAccountsRemainUsableButCannotGrow(t *testing.T) {
	f := newIdentityFixture(t)
	if _, err := f.db.runtime.Exec(f.ctx, `INSERT INTO crab.role_eligibility(human_id,role) VALUES('old-timer','Major')`); err != nil {
		t.Fatal(err)
	}
	f.setPassword("old-timer", "Bafflewick-old-26!")
	token := f.signIn("old-timer", "Bafflewick-old-26!", "")
	if !f.tokenWorks(token) {
		t.Fatal("legacy account refused")
	}
	if _, err := f.service.ProposeRoleEligibility(f.ctx, f.principal("alice"), f.id("ROLE"), "newcomer", authz.RoleMajor, true, f.now); !errors.Is(err, access.ErrIdentityRecordMissing) {
		t.Fatalf("eligibility without identity err=%v", err)
	}
}
