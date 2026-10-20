package authn

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"errors"
)

const CredentialWrapKeyBytes = 32

func WrapSecret(key, plaintext []byte) (nonce, ciphertext []byte, err error) {
	gcm, err := credentialGCM(key)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, gcm.NonceSize())
	if _, err := rand.Read(nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = gcm.Seal(nil, nonce, plaintext, nil)
	return nonce, ciphertext, nil
}

func UnwrapSecret(key, nonce, ciphertext []byte) ([]byte, error) {
	gcm, err := credentialGCM(key)
	if err != nil {
		return nil, err
	}
	if len(nonce) != gcm.NonceSize() || len(ciphertext) == 0 {
		return nil, errors.New("invalid wrapped credential material")
	}
	return gcm.Open(nil, nonce, ciphertext, nil)
}

func credentialGCM(key []byte) (cipher.AEAD, error) {
	if len(key) != CredentialWrapKeyBytes {
		return nil, errors.New("credential wrap key must be 32 bytes")
	}
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	return cipher.NewGCM(block)
}
