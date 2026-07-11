package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"errors"
	"io"
)

// ErrDecryptionFailed is returned by Decrypt for any decryption failure
// (bad base64, wrong key size, bad block alignment, padding mismatch).
// Collapsing all failure modes into a single opaque error is the standard
// defence against CBC padding oracles — the previous implementation
// returned distinct error strings for each cause, letting a network
// attacker probe ciphertext byte-by-byte.
var ErrDecryptionFailed = errors.New("aes: decryption failed")

// defaultKey is the legacy fallback used when callers pass an empty key.
// Anyone reading the source can derive the resulting key, so it MUST NOT be
// used in production. Retained only because removing it would change the
// behaviour of the deprecated PadKey / PadKeyToLength helpers.
const defaultKey = "gkit"

// ErrEmptyKey is returned by PadKeyE / PadKeyToLengthE when the caller passes
// an empty string. The previous helpers silently substituted a hard-coded
// fallback ("gkit"), which produced a publicly-known KDF.
var ErrEmptyKey = errors.New("aes: empty key")

// ErrInvalidTargetLength is returned by PadKeyToLengthE when targetLength is
// negative.
var ErrInvalidTargetLength = errors.New("aes: invalid target length")

// PadKey pads s to a valid AES key length (16, 24, or 32 bytes).
//
// Deprecated: PadKey("") silently substitutes the hard-coded "gkit" default,
// producing a publicly-known key. New code should call PadKeyE and handle
// ErrEmptyKey explicitly.
func PadKey(s string) string {
	if s == "" {
		s = defaultKey
	}
	ps := []byte(s)
	ls := len(ps)

	if ls > 32 {
		return string(ps[:32])
	}
	idx := 0
	for i := ls; !(i == 16 || i == 24 || i == 32); i++ {
		ps = append(ps, s[idx])
		idx = (idx + 1) % ls
	}

	return string(ps)
}

// PadKeyE pads s to a valid AES key length (16, 24, or 32 bytes). It returns
// ErrEmptyKey when s is empty.
func PadKeyE(s string) (string, error) {
	if s == "" {
		return "", ErrEmptyKey
	}
	return PadKey(s), nil
}

// PadKeyToLength pads s to targetLength bytes.
//
// Deprecated: PadKeyToLength("") silently substitutes "gkit". Use
// PadKeyToLengthE instead.
func PadKeyToLength(s string, targetLength int) string {
	if s == "" {
		s = defaultKey
	}
	ps := []byte(s)
	ls := len(ps)

	if ls > targetLength {
		return string(ps[:targetLength])
	}

	idx := 0
	for len(ps) < targetLength {
		ps = append(ps, s[idx])
		idx = (idx + 1) % ls
	}

	return string(ps)
}

// PadKeyToLengthE pads s to targetLength bytes. It returns ErrEmptyKey when s
// is empty and ErrInvalidTargetLength when targetLength is negative.
func PadKeyToLengthE(s string, targetLength int) (string, error) {
	if s == "" {
		return "", ErrEmptyKey
	}
	if targetLength < 0 {
		return "", ErrInvalidTargetLength
	}
	return PadKeyToLength(s, targetLength), nil
}

func Encrypt(orig string, key string) (string, error) {
	origData := []byte(orig)
	k := []byte(key)

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()
	origData = PKCS7Padding(origData, blockSize)

	iv := make([]byte, blockSize)
	if _, err := io.ReadFull(rand.Reader, iv); err != nil {
		return "", err
	}

	blockMode := cipher.NewCBCEncrypter(block, iv)
	cryted := make([]byte, len(origData))
	blockMode.CryptBlocks(cryted, origData)

	// Prepend IV to ciphertext
	result := append(iv, cryted...)
	return base64.StdEncoding.EncodeToString(result), nil
}

func Decrypt(cryted string, key string) (string, error) {
	crytedByte, err := base64.StdEncoding.DecodeString(cryted)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	k := []byte(key)

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	blockSize := block.BlockSize()

	if len(crytedByte) < blockSize {
		return "", ErrDecryptionFailed
	}
	iv := crytedByte[:blockSize]
	crytedByte = crytedByte[blockSize:]

	if len(crytedByte) == 0 || len(crytedByte)%blockSize != 0 {
		return "", ErrDecryptionFailed
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)

	orig, err = PKCS7UnPadding(orig)
	if err != nil {
		return "", ErrDecryptionFailed
	}
	return string(orig), nil
}

// PKCS7Padding 补码
func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

// PKCS7UnPadding 去码. Compares the padding bytes in constant time to
// avoid leaking the failing byte position via timing — the previous
// implementation short-circuited on the first mismatch, which combined
// with distinguishable error strings in Decrypt formed a textbook CBC
// padding oracle.
func PKCS7UnPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	unPadding := int(origData[length-1])
	if unPadding == 0 || unPadding > length {
		return nil, errors.New("invalid padding")
	}
	// Build a reference slice of `unPadding` repeated bytes and compare in
	// constant time. subtle.ConstantTimeCompare returns 1 iff both slices
	// are equal and same length; 0 otherwise.
	want := bytes.Repeat([]byte{byte(unPadding)}, unPadding)
	if subtle.ConstantTimeCompare(origData[length-unPadding:], want) != 1 {
		return nil, errors.New("invalid padding")
	}
	return origData[:(length - unPadding)], nil
}
