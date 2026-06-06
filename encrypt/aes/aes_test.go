package aes

import (
	"encoding/base64"
	"strings"
	"testing"
)

func TestPadKeyToLength(t *testing.T) {
	tests := []struct {
		name         string
		input        string
		targetLength int
		expected     string
	}{
		{
			name:         "Empty input with defaultKey",
			input:        "",
			targetLength: 8,
			expected:     "gkitgkit", // Assuming defaultKey is defined globally
		},
		{
			name:         "Input shorter than target length",
			input:        "abc",
			targetLength: 8,
			expected:     "abcabcab",
		},
		{
			name:         "Input equal to target length",
			input:        "abcdefgh",
			targetLength: 8,
			expected:     "abcdefgh",
		},
		{
			name:         "Input longer than target length",
			input:        "abcdefghijklmnopqrstuvwxyz",
			targetLength: 8,
			expected:     "abcdefgh",
		},
		{
			name:         "Empty input with zero target length",
			input:        "",
			targetLength: 0,
			expected:     "",
		},
		{
			name:         "Non-empty input with zero target length",
			input:        "abc",
			targetLength: 0,
			expected:     "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := PadKeyToLength(tt.input, tt.targetLength)
			if result != tt.expected {
				t.Errorf("expected %q, got %q", tt.expected, result)
			}
		})
	}
}

func TestEncryptDecryptRoundtrip(t *testing.T) {
	cases := []struct {
		name string
		text string
		key  string
	}{
		{"empty", "", "0123456789abcdef"},
		{"short", "hello", "0123456789abcdef"},
		{"block-aligned", "0123456789abcdef", "0123456789abcdef"},
		{"long", strings.Repeat("ab", 200), "0123456789abcdef0123456789abcdef"},
		{"utf8", "你好,世界 🌍", "0123456789abcdef"},
		{"key24", "payload", PadKeyToLength("k", 24)},
		{"key32", "payload", PadKeyToLength("k", 32)},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			enc, err := Encrypt(c.text, c.key)
			if err != nil {
				t.Fatalf("Encrypt: %v", err)
			}
			dec, err := Decrypt(enc, c.key)
			if err != nil {
				t.Fatalf("Decrypt: %v", err)
			}
			if dec != c.text {
				t.Fatalf("roundtrip mismatch: got %q, want %q", dec, c.text)
			}
		})
	}
}

func TestEncryptRandomIV(t *testing.T) {
	const text = "same plaintext"
	const key = "0123456789abcdef"
	first, err := Encrypt(text, key)
	if err != nil {
		t.Fatalf("Encrypt 1: %v", err)
	}
	second, err := Encrypt(text, key)
	if err != nil {
		t.Fatalf("Encrypt 2: %v", err)
	}
	if first == second {
		t.Fatalf("two encryptions of the same plaintext/key produced identical ciphertext; IV is not random")
	}
}

func TestEncryptInvalidKey(t *testing.T) {
	if _, err := Encrypt("data", "shortkey"); err == nil {
		t.Fatal("expected error for invalid key length")
	}
}

func TestDecryptInvalidInputs(t *testing.T) {
	const key = "0123456789abcdef"

	if _, err := Decrypt("!!!not-base64!!!", key); err == nil {
		t.Error("expected error for invalid base64")
	}

	tooShort := base64.StdEncoding.EncodeToString([]byte("short"))
	if _, err := Decrypt(tooShort, key); err == nil {
		t.Error("expected error for ciphertext shorter than block size")
	}

	notBlockMultiple := base64.StdEncoding.EncodeToString(make([]byte, 16+5))
	if _, err := Decrypt(notBlockMultiple, key); err == nil {
		t.Error("expected error for ciphertext not a multiple of block size")
	}

	enc, err := Encrypt("hello", key)
	if err != nil {
		t.Fatalf("Encrypt: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(enc)
	if err != nil {
		t.Fatalf("decode: %v", err)
	}
	raw[len(raw)-1] ^= 0xff
	tampered := base64.StdEncoding.EncodeToString(raw)
	if _, err := Decrypt(tampered, key); err == nil {
		t.Error("expected error when padding is corrupted")
	}
}

func TestPadKeyE_EmptyReturnsError(t *testing.T) {
	if _, err := PadKeyE(""); err != ErrEmptyKey {
		t.Fatalf("PadKeyE(\"\") err = %v, want ErrEmptyKey", err)
	}
	got, err := PadKeyE("abc")
	if err != nil {
		t.Fatalf("PadKeyE(\"abc\") err = %v", err)
	}
	if got != "abcabcabcabcabca" {
		t.Fatalf("PadKeyE(\"abc\") = %q", got)
	}
}

func TestPadKeyToLengthE_EmptyReturnsError(t *testing.T) {
	if _, err := PadKeyToLengthE("", 16); err != ErrEmptyKey {
		t.Fatalf("PadKeyToLengthE(\"\", 16) err = %v, want ErrEmptyKey", err)
	}
}

func TestPKCS7UnPadding(t *testing.T) {
	if _, err := PKCS7UnPadding(nil); err == nil {
		t.Error("expected error for empty input")
	}
	if _, err := PKCS7UnPadding([]byte{0x05}); err == nil {
		t.Error("expected error when unPadding exceeds length")
	}
	if _, err := PKCS7UnPadding([]byte{0x00}); err == nil {
		t.Error("expected error for zero padding byte")
	}
	bad := []byte{1, 2, 3, 4, 0xff, 0xff, 0x04}
	if _, err := PKCS7UnPadding(bad); err == nil {
		t.Error("expected error when interior pad bytes do not match")
	}
	good := []byte{1, 2, 3, 4, 4, 4, 4, 4}
	out, err := PKCS7UnPadding(good)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if string(out) != string([]byte{1, 2, 3, 4}) {
		t.Fatalf("got %v", out)
	}
}
