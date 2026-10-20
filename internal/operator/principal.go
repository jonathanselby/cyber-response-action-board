package operator

import "github.com/jonathanselby/cyber-response-action-board/internal/security/authz"

// Principal is the operator-facing identity view. Authorisation decisions remain
// in the security/application tiers; presentation only carries the resolved view.
type Principal = authz.Principal

const RoleMajor = authz.RoleMajor
