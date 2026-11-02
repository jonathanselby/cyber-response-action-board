package domain

import (
	"errors"
	"strings"
	"testing"
	"time"
)

// T-229: no single-person window can cover a reopening, however it is drawn.
func TestNoWindowCoversReopening(t *testing.T) {
	if _, ok := WindowCoverableKinds[TypeReopening]; ok {
		t.Fatal("reopening is listed as a window-coverable kind")
	}
	now := time.Date(2026, 11, 9, 1, 0, 0, 0, time.UTC)
	kinds := []string{TypeReopening}
	for k := range WindowCoverableKinds {
		kinds = append(kinds, k)
	}
	w := SinglePersonWindow{WindowID: "all-kinds", StartsAt: now.Add(-time.Hour), EndsAt: now.Add(time.Hour), Humans: []string{"alice"}, Kinds: kinds, Reason: "probe", Active: true}
	if err := ValidateSinglePersonWindowShape(w); err == nil {
		t.Fatal("a window naming reopening passed validation")
	}
}

// T-228: closure blockers are named and satisfy errors.Is for the old error.
func TestClosureBlockersAreNamed(t *testing.T) {
	err := ClosurePrerequisites(RecoveryState{Updates: []RecoveryUpdateView{{Update: RecoveryUpdate{ItemID: "ITEM-1"}}}})
	if !errors.Is(err, ErrClosurePrerequisites) {
		t.Fatalf("err=%v is not ErrClosurePrerequisites", err)
	}
	for _, want := range []string{"stand-down not confirmed", "Post-Incident Review not confirmed", "retention and hold not confirmed", "recovery item ITEM-1"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("blockers missing %q: %v", want, err)
		}
	}
	if _, err := ProposeReopening(Scope{IncidentID: "INC-1", Mode: "live", Compartment: "GENERAL"}, "R-1", "alice", time.Now(), Reopening{Reason: " "}); err == nil {
		t.Fatal("reopening without a reason accepted")
	}
}
