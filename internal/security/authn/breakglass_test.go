package authn

import "testing"

func TestBreakGlassRequiresExactlyTwoDistinctValidKeyholders(t *testing.T) {
	a, err := NewBreakGlassRecord("keyholder-a", "offline-key-a", MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	b, err := NewBreakGlassRecord("keyholder-b", "offline-key-b", MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	records := []BreakGlassRecord{a, b}
	if _, err := VerifyBreakGlass(records, map[string]string{"keyholder-a": "offline-key-a"}); err == nil {
		t.Fatal("one keyholder unexpectedly established break-glass authentication")
	}
	ids, err := VerifyBreakGlass(records, map[string]string{"keyholder-b": "offline-key-b", "keyholder-a": "offline-key-a"})
	if err != nil || len(ids) != 2 || ids[0] != "keyholder-a" || ids[1] != "keyholder-b" {
		t.Fatalf("two-key verification ids=%v err=%v", ids, err)
	}
	if _, err := VerifyBreakGlass(records, map[string]string{"keyholder-a": "offline-key-a", "keyholder-b": "wrong"}); err == nil {
		t.Fatal("invalid second key unexpectedly accepted")
	}
}
