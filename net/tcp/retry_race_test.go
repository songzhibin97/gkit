package tcp

import (
	"net"
	"sync"
	"testing"
)

// TestNilRetry_ConcurrentRealPath covers I-ff: when a caller passes a nil
// retry, Send/Recv must use a fresh per-call zero Retry rather than the
// address of a shared package global (`retry = &defaultRetry`). This drives
// the REAL Send/Recv nil-retry branch over a closed net.Pipe from many
// goroutines under -race, asserting the path is concurrency-safe and returns
// promptly (a zero retry means no retry, so a failed write returns at once).
//
// Note: the original global was a zero-value `var defaultRetry Retry`, and the
// only writes to it (`retry.Count--`, `retry.Interval=...`) sit behind a
// `Count > 0` guard that a nil caller never satisfies. The shared-global race
// is therefore latent rather than live, so this test guards the changed code
// path and its concurrency-safety, not a reproducible data race.
func TestNilRetry_ConcurrentRealPath(t *testing.T) {
	var wg sync.WaitGroup
	for i := 0; i < 16; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client, server := net.Pipe()
			_ = server.Close() // make the peer dead so writes fail
			c := NewConnByNetConn(client)
			// nil retry -> fresh zero Retry -> no retry -> prompt error.
			if err := c.Send([]byte("ping"), nil); err == nil {
				t.Error("Send to closed pipe: want error, got nil")
			}
			// Exercise the Recv nil-retry branch too; a closed pipe yields EOF
			// which Recv treats as a clean end, so we only require no hang/panic.
			_, _ = c.Recv(0, nil)
			_ = client.Close()
		}()
	}
	wg.Wait()
}
