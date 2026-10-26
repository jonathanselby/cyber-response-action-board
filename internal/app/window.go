package app

import (
	"context"
	"errors"
	"time"

	"github.com/jonathanselby/cyber-response-action-board/internal/domain"
	"github.com/jonathanselby/cyber-response-action-board/internal/ledger"
	"github.com/jonathanselby/cyber-response-action-board/internal/security/authz"
)

// WindowPolicy answers whether a confirmed single-person window lets human
// self-approve a proposal of recordType at a time.
type WindowPolicy interface {
	Permits(ctx context.Context, human, recordType string, at time.Time) (bool, error)
}

type selfApprovalKey struct{}

// WithSelfApproval marks a decision request as the maker deciding their own
// proposal, with the reason they gave. Without it, every service keeps the
// normal rule that the maker cannot decide.
func WithSelfApproval(ctx context.Context, reason string) context.Context {
	return context.WithValue(ctx, selfApprovalKey{}, reason)
}

func selfApprovalReason(ctx context.Context) (string, bool) {
	reason, ok := ctx.Value(selfApprovalKey{}).(string)
	return reason, ok
}

var ErrNoWindow = errors.New("no confirmed single-person window lets you decide your own proposal of this kind now")

// decideGoverned is the one decision step every self-approvable service
// uses. The maker deciding their own proposal is refused unless a confirmed
// window names them, covers this kind and is open now, and a reason is given.
func decideGoverned(ctx context.Context, window WindowPolicy, proposal ledger.Record, version string, principal authz.Principal, outcome string, now time.Time) (ledger.Record, error) {
	reason, self := selfApprovalReason(ctx)
	if !self || proposal.Maker != principal.HumanID {
		return domain.Decide(proposal, version, principal.HumanID, outcome, now)
	}
	if window == nil || principal.Role != authz.RoleMajor {
		return ledger.Record{}, ErrNoWindow
	}
	permitted, err := window.Permits(ctx, principal.HumanID, proposal.RecordType, now)
	if err != nil {
		return ledger.Record{}, err
	}
	if !permitted {
		return ledger.Record{}, ErrNoWindow
	}
	return domain.DecideSelf(proposal, version, principal.HumanID, outcome, reason, now)
}
