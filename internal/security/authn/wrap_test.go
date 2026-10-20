package authn

import (
	"bytes"
	"testing"
)

func TestCredentialWrapRoundTripAndTamperDetection(t *testing.T) {
	key := []byte("0123456789abcdef0123456789abcdef")
	nonce, ciphertext, err := WrapSecret(key, []byte("secret-material"))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("secret-material")) {
		t.Fatal("wrapped secret contains plaintext")
	}
	plain, err := UnwrapSecret(key, nonce, ciphertext)
	if err != nil || string(plain) != "secret-material" {
		t.Fatalf("unwrap=%q err=%v", plain, err)
	}
	ciphertext[0] ^= 0x01
	if _, err := UnwrapSecret(key, nonce, ciphertext); err == nil {
		t.Fatal("tampered wrapped credential accepted")
	}
}
