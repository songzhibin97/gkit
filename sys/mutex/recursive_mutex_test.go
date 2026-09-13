package mutex

import "testing"

func TestRecursiveMutexTryLock(t *testing.T) {
	for _, firstTry := range []bool{false, true} {
		t.Run(map[bool]string{false: "Lock", true: "TryLock"}[firstTry], func(t *testing.T) {
			var m RecursiveMutex
			if firstTry {
				if !m.TryLock() {
					t.Fatal("TryLock on zero mutex failed")
				}
			} else {
				m.Lock()
			}
			if !m.TryLock() {
				t.Fatal("owner could not recursively TryLock")
			}
			m.Lock()
			tryOther := func(want bool) {
				t.Helper()
				result := make(chan bool, 1)
				go func() {
					got := m.TryLock()
					if got {
						m.Unlock()
					}
					result <- got
				}()
				if got := <-result; got != want {
					t.Fatalf("other goroutine TryLock = %v, want %v", got, want)
				}
			}
			tryOther(false)
			m.Unlock()
			tryOther(false)
			m.Unlock()
			tryOther(false)
			m.Unlock()
			tryOther(true)
			if !m.TryLock() {
				t.Fatal("original goroutine could not reacquire")
			}
			m.Unlock()
		})
	}
}
