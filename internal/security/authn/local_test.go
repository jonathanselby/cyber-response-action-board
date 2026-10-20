package authn

import "testing"

func TestLocalPasswordWorksWithoutExternalIdentity(t *testing.T) {
	rec, err := NewPasswordRecord("alice", "correct horse battery staple", MinPBKDF2Iterations)
	if err != nil {
		t.Fatal(err)
	}
	if !VerifyPassword(rec, "correct horse battery staple") {
		t.Fatal("valid local password rejected")
	}
	if VerifyPassword(rec, "wrong") {
		t.Fatal("invalid local password accepted")
	}
	if rec.Hash == "correct horse battery staple" || rec.Salt == "correct horse battery staple" {
		t.Fatal("plaintext password stored")
	}
}
