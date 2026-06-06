package retry

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"time"
)

// jitterFraction is the maximum proportional jitter added to each sleep
// (±25%). Without jitter, synchronised callers retry in lockstep, causing
// thundering herds against a recovering backend.
const jitterFraction = 0.25

func Retry() func(ctx context.Context) {
	retryIn := 0
	fibonacci := Fibonacci()
	return func(ctx context.Context) {
		if retryIn > 0 {
			d := time.Duration(retryIn) * time.Second
			d += jitter(d)
			t := time.NewTimer(d)
			select {
			case <-ctx.Done():
				t.Stop()
			case <-t.C:
			}
		}
		retryIn = fibonacci()
	}
}

// jitter returns a random offset in ±jitterFraction * d. Uses crypto/rand
// so synchronized clients don't share a math/rand seed.
func jitter(d time.Duration) time.Duration {
	if d <= 0 {
		return 0
	}
	var buf [8]byte
	if _, err := rand.Read(buf[:]); err != nil {
		return 0
	}
	v := binary.LittleEndian.Uint64(buf[:])
	// Normalize to [-1, 1].
	signed := float64(int64(v>>1))/float64(1<<62) - 1
	return time.Duration(float64(d) * jitterFraction * signed)
}
