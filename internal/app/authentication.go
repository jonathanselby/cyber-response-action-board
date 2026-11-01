package app

import (
	"context"
	"crypto/rand"
	"encoding/base32"
	"errors"
	"net/url"
	"strings"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

var (
	// ErrAuthenticationFailed is the single refusal for any sign-in failure, so
	// a refusal never reveals whether an account exists or why it is refused.
	ErrAuthenticationFailed = errors.New("authentication failed")
	// ErrEnrolmentRequired means a valid enrolment code was presented: the
	// person must set a password (and, if external, enrol TOTP) first.
	ErrEnrolmentRequired = errors.New("enrolment required: set a password to continue")
)

// sessionSweepLimit bounds the expired sessions removed per sign-in.
const sessionSweepLimit = 100

// PrincipalSource loads authorisation state and identity state.
type PrincipalSource interface {
	PrincipalAt(context.Context, string, bool, time.Time) (authz.Principal, error)
	IdentityState(context.Context, string) (access.IdentityState, bool, error)
}

// SessionStore creates and resolves sessions.
type SessionStore interface {
	Create(context.Context, string, string, bool) (string, authn.Session, error)
	Resolve(context.Context, string) (authn.Session, bool, error)
}

// Principals is the one place a session token becomes a principal. Every
// surface (browser, JSON API, MCP, CLI print) resolves through it, so the
// account rule and clock-based expiry apply everywhere.
type Principals struct {
	Sessions SessionStore
	Source   PrincipalSource
	Now      func() time.Time
}

// ForToken returns the principal for a session token, or ok=false when the
// session is unknown or expired or the account is not usable now.
func (p Principals) ForToken(ctx context.Context, token string) (authz.Principal, authn.Session, bool, error) {
	if p.Sessions == nil || p.Source == nil || p.Now == nil {
		return authz.Principal{}, authn.Session{}, false, errors.New("principal resolution is not configured")
	}
	session, ok, err := p.Sessions.Resolve(ctx, token)
	if err != nil || !ok {
		return authz.Principal{}, authn.Session{}, false, err
	}
	now := p.Now().UTC()
	state, legacy, err := p.Source.IdentityState(ctx, session.HumanID)
	if err != nil {
		return authz.Principal{}, authn.Session{}, false, err
	}
	if !access.Usable(state, legacy, now) {
		return authz.Principal{}, authn.Session{}, false, nil
	}
	principal, err := p.Source.PrincipalAt(ctx, session.HumanID, true, now)
	if err != nil {
		return authz.Principal{}, authn.Session{}, false, err
	}
	return principal, session, true, nil
}

// Authentication owns sign-in and first-time enrolment.
type Authentication struct {
	Credentials CredentialBackend
	Sessions    SessionStore
	Identities  PrincipalSource
	// Issuer labels TOTP enrolment URIs.
	Issuer string
}

// Enrolment is what a person needs to finish setting up their account.
type Enrolment struct {
	NeedsTOTP  bool
	TOTPSecret string
	TOTPURI    string
}

// Authenticate signs a person in. External people must also present a valid
// TOTP code. A valid enrolment code returns ErrEnrolmentRequired and creates
// no session.
func (a Authentication) Authenticate(ctx context.Context, humanID, secret, totpCode string, now time.Time) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	humanID = strings.TrimSpace(humanID)
	state, cred, err := a.verifySecret(ctx, humanID, secret, now)
	if err != nil {
		return "", err
	}
	if cred.MustSetPassword {
		return "", ErrEnrolmentRequired
	}
	if state.External {
		if err := a.verifyConfirmedTOTP(ctx, humanID, totpCode, now); err != nil {
			return "", err
		}
	}
	return a.startSession(ctx, humanID, now)
}

// BeginEnrolment checks an enrolment code and, for an external person, creates
// a new pending TOTP secret to show them. Calling it again replaces the secret.
func (a Authentication) BeginEnrolment(ctx context.Context, humanID, code string, now time.Time) (Enrolment, error) {
	if err := a.ready(); err != nil {
		return Enrolment{}, err
	}
	humanID = strings.TrimSpace(humanID)
	state, cred, err := a.verifySecret(ctx, humanID, normaliseCode(code), now)
	if err != nil || !cred.MustSetPassword {
		return Enrolment{}, ErrAuthenticationFailed
	}
	if !state.External {
		return Enrolment{}, nil
	}
	secret, err := newTOTPSecret()
	if err != nil {
		return Enrolment{}, err
	}
	if err := a.Credentials.PutPendingTOTP(ctx, authn.TOTPRecord{HumanID: humanID, Secret: secret, Window: 1}); err != nil {
		return Enrolment{}, err
	}
	return Enrolment{NeedsTOTP: true, TOTPSecret: secret, TOTPURI: totpURI(a.Issuer, humanID, secret)}, nil
}

// CompleteEnrolment replaces an enrolment code with the person's own password
// (and confirms their TOTP secret if external), then signs them in.
func (a Authentication) CompleteEnrolment(ctx context.Context, humanID, code, newPassword, totpCode string, now time.Time) (string, error) {
	if err := a.ready(); err != nil {
		return "", err
	}
	humanID = strings.TrimSpace(humanID)
	state, cred, err := a.verifySecret(ctx, humanID, normaliseCode(code), now)
	if err != nil || !cred.MustSetPassword {
		return "", ErrAuthenticationFailed
	}
	if err := authn.ValidateNewPassword(newPassword); err != nil {
		return "", err
	}
	if state.External {
		record, _, found, err := a.Credentials.GetTOTP(ctx, humanID)
		if err != nil {
			return "", err
		}
		if !found {
			return "", errors.New("start enrolment again to get your authenticator code")
		}
		ok, err := authn.VerifyTOTP(record.Secret, strings.TrimSpace(totpCode), now, record.Window)
		if err != nil || !ok {
			return "", errors.New("the authenticator code was not accepted")
		}
	}
	password, err := authn.NewPasswordRecord(humanID, newPassword, authn.MinPBKDF2Iterations)
	if err != nil {
		return "", err
	}
	if err := a.Credentials.PutPassword(ctx, password); err != nil {
		return "", err
	}
	if state.External {
		if err := a.Credentials.ConfirmTOTP(ctx, humanID); err != nil {
			return "", err
		}
	}
	return a.startSession(ctx, humanID, now)
}

// verifySecret applies the account rule and checks the presented secret. Every
// failure is the same refusal.
func (a Authentication) verifySecret(ctx context.Context, humanID, secret string, now time.Time) (access.IdentityState, authn.Credential, error) {
	if humanID == "" || secret == "" {
		return access.IdentityState{}, authn.Credential{}, ErrAuthenticationFailed
	}
	state, legacy, err := a.Identities.IdentityState(ctx, humanID)
	if err != nil {
		return access.IdentityState{}, authn.Credential{}, err
	}
	if !access.Usable(state, legacy, now) {
		return access.IdentityState{}, authn.Credential{}, ErrAuthenticationFailed
	}
	cred, found, err := a.Credentials.GetCredential(ctx, humanID)
	if err != nil {
		return access.IdentityState{}, authn.Credential{}, err
	}
	if !found || !cred.Usable(now) {
		return access.IdentityState{}, authn.Credential{}, ErrAuthenticationFailed
	}
	presented := secret
	if cred.MustSetPassword {
		presented = normaliseCode(secret)
	}
	if !authn.VerifyPassword(cred.Record, presented) {
		return access.IdentityState{}, authn.Credential{}, ErrAuthenticationFailed
	}
	return state, cred, nil
}

func (a Authentication) verifyConfirmedTOTP(ctx context.Context, humanID, code string, now time.Time) error {
	record, confirmed, found, err := a.Credentials.GetTOTP(ctx, humanID)
	if err != nil {
		return err
	}
	if !found || !confirmed {
		return ErrAuthenticationFailed
	}
	ok, err := authn.VerifyTOTP(record.Secret, strings.TrimSpace(code), now, record.Window)
	if err != nil || !ok {
		return ErrAuthenticationFailed
	}
	return nil
}

func (a Authentication) startSession(ctx context.Context, humanID string, now time.Time) (string, error) {
	if err := a.Credentials.SweepExpiredSessions(ctx, now, sessionSweepLimit); err != nil {
		return "", err
	}
	token, _, err := a.Sessions.Create(ctx, humanID, "local-password", false)
	return token, err
}

func (a Authentication) ready() error {
	if a.Credentials == nil || a.Sessions == nil || a.Identities == nil {
		return errors.New("authentication is not configured")
	}
	return nil
}

// normaliseCode accepts an enrolment code with or without dashes, spaces or
// lower-case letters, so it can be read aloud and typed easily.
func normaliseCode(code string) string {
	return strings.ToUpper(strings.NewReplacer("-", "", " ", "").Replace(strings.TrimSpace(code)))
}

func newTOTPSecret() (string, error) {
	raw := make([]byte, authn.MinTOTPSecretBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw), nil
}

func totpURI(issuer, humanID, secret string) string {
	if strings.TrimSpace(issuer) == "" {
		issuer = "CRAB"
	}
	label := url.PathEscape(issuer + ":" + humanID)
	q := url.Values{"secret": {secret}, "issuer": {issuer}, "algorithm": {"SHA1"}, "digits": {"6"}, "period": {"30"}}
	return "otpauth://totp/" + label + "?" + q.Encode()
}
