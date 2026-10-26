package main

import (
	"context"
	"strings"
	"testing"

	"github.com/jonathanselby/cyber-response-action-board/internal/presentation/web"
)

// T-193: the operator runtime refuses a weak password before touching storage.
func TestBootstrapRefusesWeakPasswordBeforeStorage(t *testing.T) {
	r := &operatorRuntime{}
	for _, input := range []web.BootstrapInput{
		{PreparedBy: "alice", PreparedPassword: "alice-password-123", ConfirmedBy: "bob", ConfirmedPassword: "Bob-Password-123!"},
		{PreparedBy: "alice", PreparedPassword: "Alice-Password-123", ConfirmedBy: "bob", ConfirmedPassword: "bobpasswordnosymbol"},
	} {
		err := r.Bootstrap(context.Background(), input)
		if err == nil || !strings.Contains(err.Error(), "symbol") {
			t.Fatalf("weak password not refused by policy: %v", err)
		}
	}
}
