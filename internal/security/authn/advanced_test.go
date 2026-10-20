package authn

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"testing"
	"time"
)

func TestTOTPVectorAndSecretFloor(t *testing.T) {
	raw := []byte("12345678901234567890")
	secret := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString(raw)
	now := time.Unix(59, 0)
	if ok, err := VerifyTOTP(secret, "287082", now, 0); err != nil || !ok {
		t.Fatalf("TOTP vector ok=%v err=%v", ok, err)
	}
	short := base32.StdEncoding.WithPadding(base32.NoPadding).EncodeToString([]byte("too-short"))
	if err := ValidateTOTPSecret(short); err == nil {
		t.Fatal("short TOTP secret accepted")
	}
}

func passkeyFixture(t *testing.T) (PasskeyRecord, PasskeyAssertion) {
	t.Helper()
	priv, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	pub := string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
	challenge := base64.RawURLEncoding.EncodeToString([]byte("0123456789abcdef0123456789abcdef"))
	client, _ := json.Marshal(map[string]any{"type": "webauthn.get", "challenge": challenge, "origin": "https://crab.local"})
	rp := sha256.Sum256([]byte("crab.local"))
	authData := append([]byte{}, rp[:]...)
	authData = append(authData, 0x05, 0, 0, 0, 0)
	binary.BigEndian.PutUint32(authData[33:37], 2)
	clientHash := sha256.Sum256(client)
	message := append(append([]byte{}, authData...), clientHash[:]...)
	digest := sha256.Sum256(message)
	sig, err := ecdsa.SignASN1(rand.Reader, priv, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return PasskeyRecord{HumanID: "human-1", CredentialID: "cred-1", RPID: "crab.local", Origin: "https://crab.local", PublicKeyPEM: pub, RequireUserVerification: true, SignCount: 1}, PasskeyAssertion{ClientDataJSON: base64.RawURLEncoding.EncodeToString(client), AuthenticatorData: base64.RawURLEncoding.EncodeToString(authData), Signature: base64.RawURLEncoding.EncodeToString(sig), Challenge: challenge}
}

func TestPasskeyChecksOriginRPAndCounter(t *testing.T) {
	record, assertion := passkeyFixture(t)
	verified, err := VerifyPasskey(record, assertion)
	if err != nil || verified.SignCount != 2 || verified.CounterWarning {
		t.Fatalf("passkey verification=%+v err=%v", verified, err)
	}
	record.Origin = "https://evil.example"
	if _, err := VerifyPasskey(record, assertion); err == nil {
		t.Fatal("wrong passkey origin accepted")
	}
}

func rsaFixture(t *testing.T) (*rsa.PrivateKey, string) {
	t.Helper()
	priv, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatal(err)
	}
	der, _ := x509.MarshalPKIXPublicKey(&priv.PublicKey)
	return priv, string(pem.EncodeToMemory(&pem.Block{Type: "PUBLIC KEY", Bytes: der}))
}

func oidcToken(t *testing.T, priv *rsa.PrivateKey, claims map[string]any) string {
	t.Helper()
	header := base64.RawURLEncoding.EncodeToString([]byte(`{"alg":"RS256","typ":"JWT"}`))
	body, _ := json.Marshal(claims)
	payload := base64.RawURLEncoding.EncodeToString(body)
	digest := sha256.Sum256([]byte(header + "." + payload))
	sig, err := rsa.SignPKCS1v15(rand.Reader, priv, crypto.SHA256, digest[:])
	if err != nil {
		t.Fatal(err)
	}
	return header + "." + payload + "." + base64.RawURLEncoding.EncodeToString(sig)
}

func TestOIDCSignatureAudienceNonceAndExpiry(t *testing.T) {
	priv, pub := rsaFixture(t)
	now := time.Date(2026, 10, 20, 3, 0, 0, 0, time.UTC)
	token := oidcToken(t, priv, map[string]any{"iss": "issuer", "aud": "crab", "sub": "human-1", "exp": now.Add(time.Hour).Unix(), "nonce": "nonce-1"})
	subject, err := VerifyOIDC(token, OIDCConfig{Issuer: "issuer", Audience: "crab", PublicKeyPEM: pub}, "nonce-1", now)
	if err != nil || subject != "human-1" {
		t.Fatalf("OIDC subject=%q err=%v", subject, err)
	}
	if _, err := VerifyOIDC(token, OIDCConfig{Issuer: "issuer", Audience: "crab", PublicKeyPEM: pub}, "wrong", now); err == nil {
		t.Fatal("wrong OIDC nonce accepted")
	}
}
