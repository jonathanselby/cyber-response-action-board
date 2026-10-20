package authn

import (
	"context"
	"errors"
	"strings"
)

var ErrFederationDisabled = errors.New("federation disabled in isolate mode")

type FederationVerifier func(context.Context, string) (string, error)

// AuthenticateFederated enforces isolate mode before token parsing or provider work.
func AuthenticateFederated(ctx context.Context, isolate bool, token string, verify FederationVerifier) (string, error) {
	if isolate {
		return "", ErrFederationDisabled
	}
	if strings.TrimSpace(token) == "" || verify == nil {
		return "", errors.New("federated authentication unavailable")
	}
	humanID, err := verify(ctx, token)
	if err != nil || strings.TrimSpace(humanID) == "" {
		return "", errors.New("federated credentials rejected")
	}
	return humanID, nil
}
