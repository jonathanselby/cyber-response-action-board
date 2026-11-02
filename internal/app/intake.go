package app

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base32"
	"errors"
	"fmt"
	"strings"
	"time"
	"unicode"
	"unicode/utf8"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/access"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authn"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// Machine first-report intake (contract addendum A4). An intake key is a
// machine credential bound to one accountable human. It can create first
// reports and nothing else; it is never a session and never a principal.

// Intake limits.
const (
	IntakeRateLimit     = 60 // accepted submissions per key per IntakeRateWindow
	IntakeRateWindow    = time.Minute
	IntakeRefusalLimit  = SourceFailureLimit // refused presentations per source per FailureWindow
	maxExternalRefRunes = 128
)

var (
	// ErrIntakeRefused is the one refusal for an expired, revoked,
	// rotated-out, unknown or malformed key, and for a key whose accountable
	// person cannot act. Callers cannot tell which.
	ErrIntakeRefused = errors.New("intake key refused")
	// ErrIntakeRateLimited means the key is over its submission bound.
	ErrIntakeRateLimited = errors.New("intake rate exceeded; retry later")
	// ErrIntakeInvalid wraps a report the key may submit but that is malformed.
	ErrIntakeInvalid = errors.New("invalid first report")
	// ErrUseIntakeKeyPath is returned when an intake key issue or rotation is
	// confirmed through the generic decision path, which cannot show a secret.
	ErrUseIntakeKeyPath = errors.New("confirm an intake key from its confirmation action, which shows the secret once")
)

// IntakeSecretBackend stores and checks hashed intake key secrets.
type IntakeSecretBackend interface {
	PutIntakeSecret(ctx context.Context, keyName string, hash []byte, oldUntil, now time.Time) error
	IntakeSecretValid(ctx context.Context, keyName string, hash []byte, now time.Time) (bool, error)
}

// NewIntakeLimiters returns the per-key submission bound and the per-source
// refusal bound.
func NewIntakeLimiters() (rate, refusals *authn.FailureLimiter) {
	return &authn.FailureLimiter{MaxFailures: IntakeRateLimit, Window: IntakeRateWindow},
		&authn.FailureLimiter{MaxFailures: IntakeRefusalLimit, Window: FailureWindow}
}

// ProposeIntakeKey lets a Custodian propose issuing, rotating or revoking an
// intake key. A different Custodian confirms it.
func (s AccessService) ProposeIntakeKey(ctx context.Context, principal authz.Principal, recordID string, change access.IntakeKeyChange, now time.Time) (ledger.Record, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return ledger.Record{}, err
	}
	change.KeyName = strings.TrimSpace(change.KeyName)
	change.AccountableHuman = strings.TrimSpace(change.AccountableHuman)
	change.Reason = strings.TrimSpace(change.Reason)
	if change.ExpiresAt != nil {
		expiry := change.ExpiresAt.UTC()
		change.ExpiresAt = &expiry
	}
	if err := s.validateIntakeKey(ctx, change, now); err != nil {
		return ledger.Record{}, err
	}
	return s.proposeSystem(ctx, principal.HumanID, recordID, access.RecordIntakeKey, change, now)
}

// validateIntakeKey checks a change against the key's state and, on issue,
// that the accountable person is a usable account. It runs at proposal and
// again at confirmation.
func (s AccessService) validateIntakeKey(ctx context.Context, change access.IntakeKeyChange, now time.Time) error {
	current, err := s.Backend.IntakeKeyState(ctx, change.KeyName)
	if err != nil {
		return err
	}
	if err := access.ValidateIntakeKeyChange(change, current, now); err != nil {
		return err
	}
	if change.Action == access.IntakeKeyIssue {
		ok, err := s.usable(ctx, change.AccountableHuman, now)
		if err != nil {
			return err
		}
		if !ok {
			return access.ErrIdentityNotUsable
		}
	}
	return nil
}

// IssuedIntakeKey is what the confirming Custodian is shown once.
type IssuedIntakeKey struct {
	KeyName          string
	AccountableHuman string
	Action           string
	Secret           string // empty for a revocation
	ExpiresAt        time.Time
	PreviousUntil    *time.Time // rotation only: when the previous secret stops
}

// ConfirmIntakeKey confirms an intake key change. For issue and rotation the
// decision commits first, then a new secret is generated and only its hash is
// stored; the secret is returned once, to the confirmer. On rotation every
// previous secret stops working at the end of the overlap. If the process
// stops after the decision commits, the key has no new secret and a new
// rotation can be proposed.
func (s AccessService) ConfirmIntakeKey(ctx context.Context, principal authz.Principal, secrets IntakeSecretBackend, recordID, expectedVersion string, now time.Time) (IssuedIntakeKey, error) {
	if secrets == nil {
		return IssuedIntakeKey{}, errors.New("intake secret backend is required")
	}
	_, change, err := s.decide(ctx, principal, "", access.RecordIntakeKey, recordID, expectedVersion, "confirmed", now, true)
	if err != nil {
		return IssuedIntakeKey{}, err
	}
	k := change.IntakeKey
	state, err := s.Backend.IntakeKeyState(ctx, k.KeyName)
	if err != nil {
		return IssuedIntakeKey{}, err
	}
	out := IssuedIntakeKey{KeyName: k.KeyName, AccountableHuman: state.AccountableHuman, Action: k.Action, ExpiresAt: state.ExpiresAt}
	if k.Action == access.IntakeKeyRevoke {
		return out, nil
	}
	secret, err := newIntakeSecret()
	if err != nil {
		return IssuedIntakeKey{}, err
	}
	oldUntil := now.UTC()
	if k.Action == access.IntakeKeyRotate {
		oldUntil = oldUntil.Add(time.Duration(k.OverlapMinutes) * time.Minute)
		out.PreviousUntil = &oldUntil
	}
	if err := secrets.PutIntakeSecret(ctx, k.KeyName, hashIntakeSecret(secret), oldUntil, now); err != nil {
		return IssuedIntakeKey{}, err
	}
	out.Secret = k.KeyName + "." + secret
	return out, nil
}

// IntakeKeyView is one intake key as a Custodian sees it.
type IntakeKeyView struct {
	KeyName          string
	AccountableHuman string
	Status           string // active, expired or revoked
	ExpiresAt        time.Time
	IssuedAt         time.Time
	RotatedAt        *time.Time
	PreviousUntil    *time.Time
	CanRotate        bool
	CanRevoke        bool
}

// IntakeKeys lists intake keys for a usable Custodian. No secret or hash is
// ever returned.
func (s AccessService) IntakeKeys(ctx context.Context, principal authz.Principal, now time.Time) ([]IntakeKeyView, error) {
	if err := s.requireUsableCustodian(ctx, principal, now); err != nil {
		return nil, err
	}
	keys, overlaps, err := s.Backend.IntakeKeys(ctx, now)
	if err != nil {
		return nil, err
	}
	out := make([]IntakeKeyView, 0, len(keys))
	for _, k := range keys {
		v := IntakeKeyView{KeyName: k.KeyName, AccountableHuman: k.AccountableHuman, Status: k.Status, ExpiresAt: k.ExpiresAt, IssuedAt: k.IssuedAt, RotatedAt: k.RotatedAt}
		if until, ok := overlaps[k.KeyName]; ok {
			u := until
			v.PreviousUntil = &u
		}
		usable := access.IntakeKeyUsable(k, now)
		if k.Status == access.IntakeKeyActive && !usable {
			v.Status = "expired"
		}
		v.CanRotate, v.CanRevoke = usable, usable
		out = append(out, v)
	}
	return out, nil
}

// IntakeKeyReader reads what the intake rule needs.
type IntakeKeyReader interface {
	IntakeKeyState(context.Context, string) (access.IntakeKeyState, error)
	IdentityState(context.Context, string) (access.IdentityState, bool, error)
}

// IntakeReportBackend commits first reports and finds an earlier one by
// external reference.
type IntakeReportBackend interface {
	AppendContinuity(context.Context, ledger.Record) error
	FirstReportByRef(ctx context.Context, keyName, ref string) (string, bool, error)
}

// IntakeService accepts first reports from intake keys.
type IntakeService struct {
	Keys     IntakeKeyReader
	Secrets  IntakeSecretBackend
	Reports  IntakeReportBackend
	Rate     *authn.FailureLimiter
	Refusals *authn.FailureLimiter
}

// IntakeReport is a machine-submitted first report.
type IntakeReport struct {
	Description     string
	Reporter        string
	ContactRoute    string
	Affected        string
	ImmediateImpact string
	ReceivedAt      *time.Time
	TimeConfidence  string
	ExternalRef     string
}

// IntakeReceipt says which report holds the submission and whether this
// request created it (false: a repeat of an earlier external_ref).
type IntakeReceipt struct {
	ReportID string
	Created  bool
}

// Submit authenticates an intake key and records an ordinary first report
// attributed to the key and its accountable person. A first report never
// becomes an incident without human triage (PRD §17).
func (s IntakeService) Submit(ctx context.Context, token, source, reportID string, in IntakeReport, now time.Time) (IntakeReceipt, error) {
	if s.Keys == nil || s.Secrets == nil || s.Reports == nil || s.Rate == nil || s.Refusals == nil {
		return IntakeReceipt{}, errors.New("intake service is not configured")
	}
	key, err := s.authenticate(ctx, token, source, now)
	if err != nil {
		return IntakeReceipt{}, err
	}
	if !s.Rate.Take(key.KeyName, now) {
		return IntakeReceipt{}, ErrIntakeRateLimited
	}
	ref := strings.TrimSpace(in.ExternalRef)
	if utf8.RuneCountInString(ref) > maxExternalRefRunes || strings.IndexFunc(ref, unicode.IsControl) >= 0 {
		return IntakeReceipt{}, fmt.Errorf("%w: external_ref is at most %d printable characters", ErrIntakeInvalid, maxExternalRefRunes)
	}
	if ref != "" {
		if id, found, err := s.Reports.FirstReportByRef(ctx, key.KeyName, ref); err != nil || found {
			return IntakeReceipt{ReportID: id}, err
		}
	}
	report := domain.FirstReport{
		Reporter: strings.TrimSpace(in.Reporter), ContactRoute: strings.TrimSpace(in.ContactRoute),
		Description: strings.TrimSpace(in.Description), Affected: strings.TrimSpace(in.Affected), ImmediateImpact: strings.TrimSpace(in.ImmediateImpact),
		SourceChannel: "intake key " + key.KeyName, IntakeKey: key.KeyName, ExternalRef: ref,
		ReceivedAt: now, TimeConfidence: ledger.TimeTrusted,
	}
	if report.Reporter == "" {
		report.Reporter = key.KeyName
	}
	if in.ReceivedAt != nil {
		// A time stated by the sender is not CRAB's governed time.
		report.ReceivedAt = *in.ReceivedAt
		report.TimeConfidence = strings.TrimSpace(in.TimeConfidence)
		if report.TimeConfidence == "" {
			report.TimeConfidence = ledger.TimeUntrusted
		}
	} else if strings.TrimSpace(in.TimeConfidence) != "" {
		return IntakeReceipt{}, fmt.Errorf("%w: time_confidence needs received_at", ErrIntakeInvalid)
	}
	record, err := domain.RecordFirstReport(reportID, key.AccountableHuman, now, report)
	if err != nil {
		return IntakeReceipt{}, fmt.Errorf("%w: %v", ErrIntakeInvalid, err)
	}
	record.MakerSignature.MissingReason = "submitted with intake key " + key.KeyName + "; accountable person " + key.AccountableHuman
	if err := s.Reports.AppendContinuity(ctx, record); err != nil {
		// A concurrent repeat of the same external_ref loses the unique
		// index race; it returns the report that won.
		if ref != "" {
			if id, found, ferr := s.Reports.FirstReportByRef(ctx, key.KeyName, ref); ferr == nil && found {
				return IntakeReceipt{ReportID: id}, nil
			}
		}
		return IntakeReceipt{}, err
	}
	return IntakeReceipt{ReportID: reportID, Created: true}, nil
}

// authenticate applies the intake rule: a well-formed token, a usable key, a
// usable accountable person and a current secret. Every failure is the same
// ErrIntakeRefused and counts against the source's refusal bound.
func (s IntakeService) authenticate(ctx context.Context, token, source string, now time.Time) (access.IntakeKeyState, error) {
	if !s.Refusals.Allow(source, now) {
		return access.IntakeKeyState{}, ErrIntakeRefused
	}
	refuse := func() (access.IntakeKeyState, error) {
		s.Refusals.Failure(source, now)
		return access.IntakeKeyState{}, ErrIntakeRefused
	}
	name, secret, ok := strings.Cut(strings.TrimSpace(token), ".")
	if !ok || name == "" || secret == "" || len(token) > 200 {
		return refuse()
	}
	// The secret is checked first, so an unknown name and a known name with
	// a wrong secret take the same path and the same time.
	valid, err := s.Secrets.IntakeSecretValid(ctx, name, hashIntakeSecret(secret), now)
	if err != nil {
		return access.IntakeKeyState{}, err
	}
	if !valid {
		return refuse()
	}
	key, err := s.Keys.IntakeKeyState(ctx, name)
	if err != nil {
		return access.IntakeKeyState{}, err
	}
	if !access.IntakeKeyUsable(key, now) {
		return refuse()
	}
	identity, legacy, err := s.Keys.IdentityState(ctx, key.AccountableHuman)
	if err != nil {
		return access.IntakeKeyState{}, err
	}
	if !access.Usable(identity, legacy, now) {
		return refuse()
	}
	return key, nil
}

// newIntakeSecret returns 160 random bits, base32, lower case.
func newIntakeSecret() (string, error) {
	raw := make([]byte, 20)
	if _, err := rand.Read(raw); err != nil {
		return "", err
	}
	return strings.ToLower(base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)), nil
}

// hashIntakeSecret is SHA-256: the secret is random and high-entropy, so a
// slow password hash adds nothing and the hash can be an index key.
func hashIntakeSecret(secret string) []byte {
	sum := sha256.Sum256([]byte(secret))
	return sum[:]
}
