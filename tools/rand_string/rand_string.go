package rand_string

import (
	"crypto/rand"
	"encoding/binary"
)

const (
	letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
	numberBytes = "0123456789"
)

// RandomLetter returns a cryptographically-random string of n characters drawn
// from [0-9a-zA-Z]. Suitable for tokens, session IDs, and similar secrets.
func RandomLetter(n int) string {
	return random(letterBytes, n)
}

// RandomInt returns a cryptographically-random string of n decimal digits.
func RandomInt(n int) string {
	return random(numberBytes, n)
}

// RandomBytes returns a cryptographically-random string of n characters drawn
// from the supplied alphabet.
func RandomBytes(bytes string, n int) string {
	return random(bytes, n)
}

func random(alphabet string, n int) string {
	if n <= 0 || len(alphabet) == 0 {
		return ""
	}
	out := make([]byte, n)
	// 8 random bytes per output character gives plenty of bits to reject
	// modulo-biased samples; one batch up front is one syscall.
	buf := make([]byte, n*8)
	if _, err := rand.Read(buf); err != nil {
		// crypto/rand.Read on a healthy OS does not fail; if it does we
		// would rather panic than emit predictable output.
		panic("rand_string: crypto/rand.Read failed: " + err.Error())
	}
	alen := uint64(len(alphabet))
	// largest multiple of alen that fits in uint64; anything above gets rejected
	// to avoid modulo bias.
	maxUnbiased := (^uint64(0) / alen) * alen
	bi := 0
	for i := 0; i < n; {
		if bi+8 > len(buf) {
			if _, err := rand.Read(buf); err != nil {
				panic("rand_string: crypto/rand.Read failed: " + err.Error())
			}
			bi = 0
		}
		v := binary.LittleEndian.Uint64(buf[bi : bi+8])
		bi += 8
		if v >= maxUnbiased {
			continue
		}
		out[i] = alphabet[v%alen]
		i++
	}
	return string(out)
}
