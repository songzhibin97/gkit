package aes

import (
	"encoding/base64"
	"errors"
	"testing"
)

func TestGCMRoundTripUsesRandomNonce(t *testing.T) {
	const (
		plaintext = "authenticated payload"
		key       = "0123456789abcdef0123456789abcdef"
	)
	first, err := EncryptGCM(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	second, err := EncryptGCM(plaintext, key)
	if err != nil {
		t.Fatal(err)
	}
	if first == second {
		t.Fatal("two GCM encryptions reused the same nonce")
	}
	for _, ciphertext := range []string{first, second} {
		got, err := DecryptGCM(ciphertext, key)
		if err != nil {
			t.Fatal(err)
		}
		if got != plaintext {
			t.Fatalf("DecryptGCM() = %q, want %q", got, plaintext)
		}
	}
}

func TestGCMRejectsWrongKeyAndEveryPayloadRegionTamper(t *testing.T) {
	const key = "0123456789abcdef0123456789abcdef"
	ciphertext, err := EncryptGCM("authenticated payload long enough for regions", key)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := DecryptGCM(ciphertext, "fedcba9876543210fedcba9876543210"); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("wrong-key error = %v, want ErrDecryptionFailed", err)
	}

	raw, err := base64.StdEncoding.DecodeString(ciphertext)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{0, len(raw) / 2, len(raw) - 1} {
		tampered := append([]byte(nil), raw...)
		tampered[index] ^= 0x01
		encoded := base64.StdEncoding.EncodeToString(tampered)
		if _, err := DecryptGCM(encoded, key); !errors.Is(err, ErrDecryptionFailed) {
			t.Fatalf("tamper at byte %d error = %v, want ErrDecryptionFailed", index, err)
		}
	}
	if _, err := DecryptGCM("not-base64", key); !errors.Is(err, ErrDecryptionFailed) {
		t.Fatalf("invalid base64 error = %v, want ErrDecryptionFailed", err)
	}
}
