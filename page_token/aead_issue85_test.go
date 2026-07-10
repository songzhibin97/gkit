package page_token

import (
	"encoding/base64"
	"errors"
	"fmt"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/encrypt/aes"
)

func TestPageTokenRejectsAuthenticatedCiphertextTampering(t *testing.T) {
	pageToken, err := NewTokenGenerateE("orders", SetSalt("issue-85-strong-salt"))
	if err != nil {
		t.Fatal(err)
	}
	encoded := pageToken.ForIndex(42)
	if encoded == "" {
		t.Fatal("ForIndex returned an empty token")
	}
	raw, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range []int{0, len(raw) / 2, len(raw) - 1} {
		tampered := append([]byte(nil), raw...)
		tampered[index] ^= 0x01
		_, err := pageToken.GetIndex(base64.StdEncoding.EncodeToString(tampered))
		if !errors.Is(err, ErrInvalidToken) {
			t.Fatalf("tamper at byte %d error = %v, want ErrInvalidToken", index, err)
		}
	}
}

func TestPageTokenRejectsLegacyCBCToken(t *testing.T) {
	pageToken, err := NewTokenGenerateE("orders", SetSalt("issue-85-strong-salt"))
	if err != nil {
		t.Fatal(err)
	}
	implementation := pageToken.(*token)
	legacyPlaintext := fmt.Sprintf("%s%s%s:%d", implementation.resourceIdentification, resourceDelim, time.Now().Format(layout), 42)
	legacyToken, err := aes.Encrypt(legacyPlaintext, implementation.salt)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pageToken.GetIndex(legacyToken); !errors.Is(err, ErrInvalidToken) {
		t.Fatalf("legacy CBC token error = %v, want ErrInvalidToken", err)
	}
}
