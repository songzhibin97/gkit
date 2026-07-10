package aes

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"io"
)

// EncryptGCM encrypts plaintext with AES-GCM and a fresh random nonce. The
// returned base64 payload is nonce || ciphertext || authentication tag.
func EncryptGCM(plaintext string, key string) (string, error) {
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", err
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}
	nonce := make([]byte, aead.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}
	sealed := aead.Seal(nil, nonce, []byte(plaintext), nil)
	payload := make([]byte, 0, len(nonce)+len(sealed))
	payload = append(payload, nonce...)
	payload = append(payload, sealed...)
	return base64.StdEncoding.EncodeToString(payload), nil
}

// DecryptGCM authenticates and decrypts a payload produced by EncryptGCM.
// All malformed or unauthenticated inputs return ErrDecryptionFailed.
func DecryptGCM(encoded string, key string) (string, error) {
	payload, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	block, err := aes.NewCipher([]byte(key))
	if err != nil {
		return "", ErrDecryptionFailed
	}
	aead, err := cipher.NewGCM(block)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	nonceSize := aead.NonceSize()
	if len(payload) < nonceSize+aead.Overhead() {
		return "", ErrDecryptionFailed
	}
	nonce := payload[:nonceSize]
	ciphertext := payload[nonceSize:]
	plaintext, err := aead.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	return string(plaintext), nil
}
