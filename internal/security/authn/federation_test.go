package authn

import (
	"context"
	"errors"
	"testing"
)

func TestIsolateModeRefusesFederationBeforeVerification(t *testing.T) {
	called := false
	verify := func(context.Context, string) (string, error) {
		called = true
		return "human-1", nil
	}
	if _, err := AuthenticateFederated(context.Background(), true, "not-even-a-token", verify); !errors.Is(err, ErrFederationDisabled) {
		t.Fatalf("isolate error=%v", err)
	}
	if called {
		t.Fatal("federation verifier called while isolate mode active")
	}
}
