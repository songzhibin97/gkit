package aes

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"errors"
	"io"
)

// defaultKey is the legacy fallback used when callers pass an empty key.
// Anyone reading the source can derive the resulting key, so it MUST NOT be
// used in production. Retained only because removing it would change the
// behaviour of the deprecated PadKey / PadKeyToLength helpers.
const defaultKey = "gkit"

// ErrEmptyKey is returned by PadKeyE / PadKeyToLengthE when the caller passes
// an empty string. The previous helpers silently substituted a hard-coded
// fallback ("gkit"), which produced a publicly-known KDF.
var ErrEmptyKey = errors.New("aes: empty key")

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
// is empty.
func PadKeyToLengthE(s string, targetLength int) (string, error) {
	if s == "" {
		return "", ErrEmptyKey
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
		return "", err
	}
	k := []byte(key)

	block, err := aes.NewCipher(k)
	if err != nil {
		return "", err
	}
	blockSize := block.BlockSize()

	if len(crytedByte) < blockSize {
		return "", errors.New("ciphertext too short")
	}
	iv := crytedByte[:blockSize]
	crytedByte = crytedByte[blockSize:]

	if len(crytedByte)%blockSize != 0 {
		return "", errors.New("ciphertext is not a multiple of the block size")
	}

	blockMode := cipher.NewCBCDecrypter(block, iv)
	orig := make([]byte, len(crytedByte))
	blockMode.CryptBlocks(orig, crytedByte)

	orig, err = PKCS7UnPadding(orig)
	if err != nil {
		return "", err
	}
	return string(orig), nil
}

// PKCS7Padding 补码
func PKCS7Padding(ciphertext []byte, blocksize int) []byte {
	padding := blocksize - len(ciphertext)%blocksize
	padText := bytes.Repeat([]byte{byte(padding)}, padding)
	return append(ciphertext, padText...)
}

// PKCS7UnPadding 去码
func PKCS7UnPadding(origData []byte) ([]byte, error) {
	length := len(origData)
	if length == 0 {
		return nil, errors.New("empty data")
	}
	unPadding := int(origData[length-1])
	if unPadding == 0 || unPadding > length {
		return nil, errors.New("invalid padding")
	}
	for i := length - unPadding; i < length; i++ {
		if int(origData[i]) != unPadding {
			return nil, errors.New("invalid padding")
		}
	}
	return origData[:(length - unPadding)], nil
}
