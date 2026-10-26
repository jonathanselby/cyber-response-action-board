package authn

import "testing"

// T-193: one rule for human-chosen passwords.
func TestValidateNewPasswordRequiresLengthCaseAndSymbol(t *testing.T) {
	for _, bad := range []string{
		"",
		"Short!1",                // too short
		"alllowercase-password",  // no upper case
		"ALLUPPERCASE-PASSWORD",  // no lower case
		"NoSymbolsHere123",       // no symbol
		"Spaces Are Not Symbols", // space is not a symbol
	} {
		if err := ValidateNewPassword(bad); err == nil {
			t.Errorf("password %q accepted", bad)
		}
	}
	for _, good := range []string{"Correct-Horse-1", "Ünïcødé_Pässwörd", "a1B!a1B!a1B!"} {
		if err := ValidateNewPassword(good); err != nil {
			t.Errorf("password %q refused: %v", good, err)
		}
	}
}

// Generated break-glass keys are not human-chosen and keep their own rule.
func TestBreakGlassKeysAreNotSubjectToHumanPasswordRule(t *testing.T) {
	if _, err := NewPasswordRecord("keyholder", "generatedkeymaterialwithoutsymbols", MinPBKDF2Iterations); err != nil {
		t.Fatalf("generated key refused: %v", err)
	}
}
