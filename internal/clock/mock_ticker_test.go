package clock

import (
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestNewMockTickerRejectsNonPositivePeriodSynchronously(t *testing.T) {
	for _, period := range []time.Duration{0, -time.Nanosecond} {
		t.Run(period.String(), func(t *testing.T) {
			want := capturePanic(t, func() { time.NewTicker(period) })
			require.Equal(t, want, capturePanic(t, func() { NewRealTicker(period) }))
			require.Equal(t, want, capturePanic(t, func() { NewMockTicker(period) }))
		})
	}
}

func TestMockTickerEmitsForPositivePeriod(t *testing.T) {
	mockClock := NewMockClock()
	SetClock(mockClock)
	t.Cleanup(func() { SetClock(NewRealClock()) })

	period := time.Second
	ticker := NewMockTicker(period)
	t.Cleanup(ticker.Stop)

	mockClock.Sleep(period)
	ticker.check()

	select {
	case tick := <-ticker.C():
		require.Equal(t, mockClock.Now(), tick)
	case <-time.After(time.Second):
		t.Fatal("positive-period mock ticker did not emit")
	}
}

func TestMockTickerStopIsIdempotentAndTerminates(t *testing.T) {
	ticker := NewMockTicker(time.Hour)

	ticker.Stop()
	ticker.Stop()

	select {
	case <-ticker.done:
	case <-time.After(time.Second):
		t.Fatal("mock ticker background loop did not terminate")
	}

	select {
	case tick := <-ticker.C():
		t.Fatalf("stopped mock ticker emitted an unexpected tick: %v", tick)
	case <-time.After(5 * time.Millisecond):
	}
}

func TestMockTickerStopIsSafeConcurrently(t *testing.T) {
	ticker := NewMockTicker(time.Hour)

	const callers = 100
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(callers)
	for i := 0; i < callers; i++ {
		go func() {
			defer wg.Done()
			<-start
			ticker.Stop()
		}()
	}

	close(start)
	wg.Wait()

	select {
	case <-ticker.done:
	case <-time.After(time.Second):
		t.Fatal("mock ticker background loop did not terminate")
	}
}

func TestMockTickerStopWaitsForPendingCheck(t *testing.T) {
	mockClock := NewMockClock()
	SetClock(mockClock)
	t.Cleanup(func() { SetClock(NewRealClock()) })

	ticker := NewMockTicker(time.Second)
	ticker.lock.Lock()
	mockClock.Sleep(time.Second)

	stopReturned := make(chan struct{})
	go func() {
		ticker.Stop()
		close(stopReturned)
	}()

	select {
	case <-ticker.stop:
	case <-time.After(time.Second):
		ticker.lock.Unlock()
		t.Fatal("Stop did not signal the background loop")
	}

	returnedBeforePendingCheck := false
	select {
	case <-stopReturned:
		returnedBeforePendingCheck = true
	default:
	}
	ticker.lock.Unlock()

	require.False(t, returnedBeforePendingCheck, "Stop returned before the pending check completed")
	select {
	case <-stopReturned:
	case <-time.After(time.Second):
		t.Fatal("Stop did not return after the pending check completed")
	}
}

func capturePanic(t *testing.T, f func()) (value any) {
	t.Helper()
	defer func() {
		value = recover()
		if value == nil {
			t.Fatal("expected a synchronous panic")
		}
	}()
	f()
	return fmt.Errorf("unreachable")
}
