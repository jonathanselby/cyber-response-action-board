package authn

import (
	"bytes"
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rsa"
	"crypto/sha1"
	"crypto/sha256"
	"crypto/subtle"
	"crypto/x509"
	"encoding/base32"
	"encoding/base64"
	"encoding/binary"
	"encoding/json"
	"encoding/pem"
	"errors"
	"strings"
	"time"
)

const (
	MinTOTPSecretBytes      = 20
	MaxTOTPWindow           = 2
	MaxOIDCClockSkewSeconds = 300
)

type TOTPRecord struct {
	HumanID string
	Secret  string
	Window  int
}

type TOTPStore interface {
	GetTOTPRecord(context.Context, string) (TOTPRecord, bool, error)
}

func AuthenticateTOTP(ctx context.Context, store TOTPStore, humanID, code string, now time.Time) (bool, error) {
	if store == nil || humanID == "" {
		return false, nil
	}
	record, ok, err := store.GetTOTPRecord(ctx, humanID)
	if err != nil || !ok {
		return false, err
	}
	return VerifyTOTP(record.Secret, code, now, record.Window)
}

func VerifyTOTP(secret, code string, now time.Time, window int) (bool, error) {
	if window < 0 || window > MaxTOTPWindow || now.IsZero() || now.Unix() < 0 {
		return false, errors.New("invalid TOTP parameters")
	}
	if len(code) != 6 {
		return false, nil
	}
	for i := range code {
		if code[i] < '0' || code[i] > '9' {
			return false, nil
		}
	}
	raw, err := decodeTOTPSecret(secret)
	if err != nil {
		return false, err
	}
	counter := now.Unix() / 30
	for delta := -window; delta <= window; delta++ {
		if counter+int64(delta) < 0 {
			continue
		}
		want := totp(raw, uint64(counter+int64(delta)))
		if subtle.ConstantTimeCompare([]byte(want), []byte(code)) == 1 {
			return true, nil
		}
	}
	return false, nil
}

func ValidateTOTPSecret(secret string) error {
	_, err := decodeTOTPSecret(secret)
	return err
}

func decodeTOTPSecret(secret string) ([]byte, error) {
	raw, err := base32.StdEncoding.WithPadding(base32.NoPadding).DecodeString(strings.ToUpper(strings.TrimSpace(secret)))
	if err != nil || len(raw) < MinTOTPSecretBytes {
		return nil, errors.New("invalid or short TOTP secret")
	}
	return raw, nil
}

func totp(secret []byte, counter uint64) string {
	var b [8]byte
	binary.BigEndian.PutUint64(b[:], counter)
	mac := hmac.New(sha1.New, secret)
	_, _ = mac.Write(b[:])
	h := mac.Sum(nil)
	offset := h[len(h)-1] & 0x0f
	n := (uint32(h[offset])&0x7f)<<24 | uint32(h[offset+1])<<16 | uint32(h[offset+2])<<8 | uint32(h[offset+3])
	n %= 1000000
	out := []byte("000000")
	for i := 5; i >= 0; i-- {
		out[i] = byte('0' + n%10)
		n /= 10
	}
	return string(out)
}

type PasskeyRecord struct {
	HumanID                  string
	CredentialID             string
	RPID                     string
	Origin                   string
	PublicKeyPEM             string
	RequireUserVerification  bool
	SignCount                uint32
}

type PasskeyAssertion struct {
	ClientDataJSON     string
	AuthenticatorData string
	Signature         string
	Challenge         string
}

type PasskeyVerification struct {
	SignCount      uint32
	CounterWarning bool
}

func VerifyPasskey(record PasskeyRecord, assertion PasskeyAssertion) (PasskeyVerification, error) {
	if record.HumanID == "" || record.CredentialID == "" || record.RPID == "" || record.Origin == "" || assertion.Challenge == "" {
		return PasskeyVerification{}, errors.New("passkey verification parameters missing")
	}
	clientData, err := base64.RawURLEncoding.DecodeString(assertion.ClientDataJSON)
	if err != nil {
		return PasskeyVerification{}, err
	}
	var client struct {
		Type        string `json:"type"`
		Challenge   string `json:"challenge"`
		Origin      string `json:"origin"`
		CrossOrigin bool   `json:"crossOrigin,omitempty"`
	}
	if err := json.Unmarshal(clientData, &client); err != nil || client.Type != "webauthn.get" || client.Challenge != assertion.Challenge || client.Origin != record.Origin || client.CrossOrigin {
		return PasskeyVerification{}, errors.New("passkey client data rejected")
	}
	authData, err := base64.RawURLEncoding.DecodeString(assertion.AuthenticatorData)
	if err != nil || len(authData) < 37 {
		return PasskeyVerification{}, errors.New("invalid authenticator data")
	}
	rpHash := sha256.Sum256([]byte(record.RPID))
	if !bytes.Equal(authData[:32], rpHash[:]) || authData[32]&0x01 == 0 {
		return PasskeyVerification{}, errors.New("relying party or user-presence check failed")
	}
	if record.RequireUserVerification && authData[32]&0x04 == 0 {
		return PasskeyVerification{}, errors.New("user verification required")
	}
	pub, err := parseECDSA(record.PublicKeyPEM)
	if err != nil {
		return PasskeyVerification{}, err
	}
	sig, err := base64.RawURLEncoding.DecodeString(assertion.Signature)
	if err != nil {
		return PasskeyVerification{}, err
	}
	clientHash := sha256.Sum256(clientData)
	msg := make([]byte, 0, len(authData)+len(clientHash))
	msg = append(msg, authData...)
	msg = append(msg, clientHash[:]...)
	digest := sha256.Sum256(msg)
	if !ecdsa.VerifyASN1(pub, digest[:], sig) {
		return PasskeyVerification{}, errors.New("passkey signature rejected")
	}
	count := binary.BigEndian.Uint32(authData[33:37])
	warning := (record.SignCount != 0 || count != 0) && count <= record.SignCount
	return PasskeyVerification{SignCount: count, CounterWarning: warning}, nil
}

func ValidatePasskeyRecord(record PasskeyRecord) error {
	if record.HumanID == "" || record.CredentialID == "" || record.RPID == "" || record.Origin == "" {
		return errors.New("passkey record incomplete")
	}
	_, err := parseECDSA(record.PublicKeyPEM)
	return err
}

func parseECDSA(value string) (*ecdsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("invalid ECDSA PEM")
	}
	key, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, err
	}
	pub, ok := key.(*ecdsa.PublicKey)
	if !ok || pub.Curve != elliptic.P256() {
		return nil, errors.New("passkey key must be ECDSA P-256")
	}
	return pub, nil
}

type OIDCConfig struct {
	Issuer           string
	Audience         string
	TrustedAudiences []string
	PublicKeyPEM     string
	ClockSkewSeconds int
}

func VerifyOIDC(token string, config OIDCConfig, expectedNonce string, now time.Time) (string, error) {
	if err := validateOIDCConfig(config); err != nil {
		return "", err
	}
	parts := strings.Split(token, ".")
	if len(parts) != 3 {
		return "", errors.New("invalid OIDC token")
	}
	var header struct {
		Alg string `json:"alg"`
	}
	if err := decodeJWTPart(parts[0], &header); err != nil || header.Alg != "RS256" {
		return "", errors.New("unexpected OIDC algorithm")
	}
	var claims struct {
		Issuer   string          `json:"iss"`
		Audience json.RawMessage `json:"aud"`
		Subject  string          `json:"sub"`
		Expires  int64           `json:"exp"`
		NotBefore int64          `json:"nbf,omitempty"`
		Nonce    string          `json:"nonce,omitempty"`
		AZP      string          `json:"azp,omitempty"`
	}
	if err := decodeJWTPart(parts[1], &claims); err != nil {
		return "", err
	}
	pub, err := parseRSA(config.PublicKeyPEM)
	if err != nil {
		return "", err
	}
	sig, err := base64.RawURLEncoding.DecodeString(parts[2])
	if err != nil {
		return "", err
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	if err := rsa.VerifyPKCS1v15(pub, crypto.SHA256, digest[:], sig); err != nil {
		return "", errors.New("OIDC signature rejected")
	}
	if claims.Issuer != config.Issuer || claims.Subject == "" || claims.Expires == 0 {
		return "", errors.New("OIDC claims rejected")
	}
	audiences, err := parseAudiences(claims.Audience)
	if err != nil || !contains(audiences, config.Audience) {
		return "", errors.New("OIDC audience rejected")
	}
	trusted := map[string]bool{config.Audience: true}
	for _, audience := range config.TrustedAudiences {
		trusted[audience] = true
	}
	for _, audience := range audiences {
		if !trusted[audience] {
			return "", errors.New("OIDC contains untrusted audience")
		}
	}
	if len(audiences) > 1 && claims.AZP == "" {
		return "", errors.New("OIDC azp required for multiple audiences")
	}
	if claims.AZP != "" && claims.AZP != config.Audience {
		return "", errors.New("OIDC azp rejected")
	}
	if expectedNonce != "" && claims.Nonce != expectedNonce {
		return "", errors.New("OIDC nonce rejected")
	}
	skew := int64(config.ClockSkewSeconds)
	if now.IsZero() || now.Unix() > claims.Expires+skew {
		return "", errors.New("OIDC token expired")
	}
	if claims.NotBefore != 0 && now.Unix()+skew < claims.NotBefore {
		return "", errors.New("OIDC token not yet valid")
	}
	return claims.Subject, nil
}

func validateOIDCConfig(config OIDCConfig) error {
	if config.Issuer == "" || config.Audience == "" || config.PublicKeyPEM == "" {
		return errors.New("OIDC issuer, audience and pinned public key are required")
	}
	if config.ClockSkewSeconds < 0 || config.ClockSkewSeconds > MaxOIDCClockSkewSeconds {
		return errors.New("OIDC clock skew out of range")
	}
	seen := map[string]bool{config.Audience: true}
	for _, audience := range config.TrustedAudiences {
		if audience == "" || seen[audience] {
			return errors.New("invalid or duplicate trusted OIDC audience")
		}
		seen[audience] = true
	}
	_, err := parseRSA(config.PublicKeyPEM)
	return err
}

func parseRSA(value string) (*rsa.PublicKey, error) {
	block, _ := pem.Decode([]byte(value))
	if block == nil {
		return nil, errors.New("invalid RSA PEM")
	}
	if key, err := x509.ParsePKIXPublicKey(block.Bytes); err == nil {
		if pub, ok := key.(*rsa.PublicKey); ok && pub.N.BitLen() >= 2048 {
			return pub, nil
		}
	}
	if pub, err := x509.ParsePKCS1PublicKey(block.Bytes); err == nil && pub.N.BitLen() >= 2048 {
		return pub, nil
	}
	return nil, errors.New("invalid or short RSA public key")
}

func decodeJWTPart(value string, target any) error {
	raw, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return err
	}
	return json.Unmarshal(raw, target)
}

func parseAudiences(raw json.RawMessage) ([]string, error) {
	var one string
	if json.Unmarshal(raw, &one) == nil {
		if one == "" {
			return nil, errors.New("audience empty")
		}
		return []string{one}, nil
	}
	var many []string
	if err := json.Unmarshal(raw, &many); err != nil || len(many) == 0 {
		return nil, errors.New("invalid audience")
	}
	seen := map[string]bool{}
	for _, audience := range many {
		if audience == "" || seen[audience] {
			return nil, errors.New("invalid or duplicate audience")
		}
		seen[audience] = true
	}
	return many, nil
}

func contains(values []string, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}
