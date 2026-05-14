package rand_string

import (
	"math/rand"
	"sync"
	"time"
)

const letterBytes = "0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
const numberBytes = "0123456789"

const (
	letterIdxBits = 6                    // 6 bits to represent a letter index
	letterIdxMask = 1<<letterIdxBits - 1 // All 1-bits, as many as letterIdxBits
	letterIdxMax  = 63 / letterIdxBits   // # of letter indices fitting in 63 bits
)

var (
	src   = rand.NewSource(time.Now().UnixNano())
	srcMu sync.Mutex
)

func RandomLetter(n int) string {
	return random(letterBytes, n)
}

func RandomInt(n int) string {
	return random(numberBytes, n)
}

func RandomBytes(bytes string, n int) string  {
	return random(bytes, n)
}

func random(bytes string, n int) string {
	b := make([]byte, n)
	srcMu.Lock()
	cache, remain := src.Int63(), letterIdxMax
	srcMu.Unlock()
	for i := n - 1; i >= 0; {
		if remain == 0 {
			srcMu.Lock()
			cache, remain = src.Int63(), letterIdxMax
			srcMu.Unlock()
		}
		if idx := int(cache & letterIdxMask); idx < len(bytes) {
			b[i] = bytes[idx]
			i--
		}
		cache >>= letterIdxBits
		remain--
	}

	return string(b)
}
