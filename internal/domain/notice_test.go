package domain

import (
	"strings"
	"testing"
	"time"
)

func TestNoticeTextCarriesNoContent(t *testing.T) {
	for _, kind := range []string{NoticeAssignment, NoticeDecision, NoticeDeadline, NoticeEscalation, NoticeAccess} {
		with, without := NoticeText(kind, "INC-7", 3), NoticeText(kind, "", 3)
		if !strings.Contains(with, "INC-7") || strings.Contains(without, "INC") || !strings.HasSuffix(with, "Sign in to CRAB.") || strings.Contains(with, "http") {
			t.Fatalf("%s: %q / %q", kind, with, without)
		}
	}
	if strings.Contains(SharedEscalationText(), "INC") {
		t.Fatal("shared notice carries an incident ID")
	}
}

func TestMaskAddress(t *testing.T) {
	cases := map[string]string{"jane@forensic.example": "j…@forensic.example", "+61400000789": "…789", "12": "…"}
	for in, want := range cases {
		if got := MaskAddress(in); got != want {
			t.Fatalf("MaskAddress(%q)=%q want %q", in, got, want)
		}
	}
}

func TestNoticeValidation(t *testing.T) {
	now := time.Now()
	ok := Notice{NoticeID: "n", Kind: NoticeAssignment, RecipientHumanID: "carol", Status: NoticeNoRoute, SourceID: "s", SourceAt: now, AttemptedAt: now}
	if err := ValidateNotice(ok); err != nil {
		t.Fatal(err)
	}
	bad := []Notice{ok, ok, ok, ok}
	bad[0].SharedChannel = "teams"                              // both recipient and shared
	bad[1].RecipientHumanID, bad[1].SharedChannel = "", "teams" // shared, but not escalation
	bad[2].Detail = "rejected carol@forensic.example"
	bad[3].Kind = "comment"
	for i, n := range bad {
		if ValidateNotice(n) == nil {
			t.Fatalf("bad notice %d accepted", i)
		}
	}
}
