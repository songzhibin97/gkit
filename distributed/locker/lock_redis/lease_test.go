package lock_redis

import (
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/locker"
)

// fakeLocker is a minimal in-memory Locker. Renew succeeds for the first
// `succeedFor` calls, then errors forever — letting us drive NewLease's
// max-retry path without Redis. Set succeedFor=0 for "always fail".
type fakeLocker struct {
	mu           sync.Mutex
	held         map[string]string // key → mark
	succeedFor   int
	renewCalls   atomic.Int64
	unlockCalled atomic.Bool
}

var _ locker.Locker = (*fakeLocker)(nil)

func (f *fakeLocker) Lock(key string, _ int, mark string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held == nil {
		f.held = make(map[string]string)
	}
	if _, ok := f.held[key]; ok {
		return errors.New("already held")
	}
	f.held[key] = mark
	return nil
}

func (f *fakeLocker) UnLock(key, mark string) error {
	f.unlockCalled.Store(true)
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held[key] != mark {
		return errors.New("mark mismatch")
	}
	delete(f.held, key)
	return nil
}

func (f *fakeLocker) Renew(key string, _ int, mark string) error {
	calls := f.renewCalls.Add(1)
	if calls > int64(f.succeedFor) {
		return errors.New("renew failed")
	}
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.held[key] != mark {
		return errors.New("mark mismatch")
	}
	return nil
}

func TestNewLease_LostFiresOnPermanentRenewFailure(t *testing.T) {
	lock := &fakeLocker{succeedFor: 0} // every Renew fails
	lease, err := NewLease(lock, "k", 30,
		SetLeaseInterval(10*time.Millisecond),
		SetLeaseMaxRetry(2),
	)
	if err != nil {
		t.Fatalf("NewLease: %v", err)
	}
	select {
	case <-lease.Lost():
		// expected
	case <-time.After(time.Second):
		t.Fatal("Lost() never fired — caller would never learn lease was abandoned")
	}
	// Cancel after Lost should be a no-op (and not panic).
	if err := lease.Cancel(); err != nil {
		t.Fatalf("Cancel after Lost: %v", err)
	}
}

func TestNewLease_HappyPath(t *testing.T) {
	lock := &fakeLocker{succeedFor: 100} // plenty of headroom
	lease, err := NewLease(lock, "k", 100,
		SetLeaseInterval(20*time.Millisecond),
		SetLeaseMaxRetry(5),
	)
	if err != nil {
		t.Fatalf("NewLease: %v", err)
	}
	time.Sleep(60 * time.Millisecond)
	select {
	case <-lease.Lost():
		t.Fatal("Lost() fired unexpectedly on happy path")
	default:
	}
	if err := lease.Cancel(); err != nil {
		t.Fatalf("Cancel: %v", err)
	}
	if !lock.unlockCalled.Load() {
		t.Fatal("Cancel did not call UnLock")
	}
}
