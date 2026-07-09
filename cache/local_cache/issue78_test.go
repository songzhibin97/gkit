package local_cache

import (
	"fmt"
	"sync"
	"testing"
	"time"
)

func TestIteratorDoesNotDeleteFreshReplacement(t *testing.T) {
	const keyCount = 32
	expired := make(map[string]Iterator, keyCount)
	for i := 0; i < keyCount; i++ {
		expired[fmt.Sprintf("k-%02d", i)] = Iterator{
			Val:    "expired",
			Expire: time.Now().Add(-time.Second).UnixNano(),
		}
	}
	firstDelete := make(chan struct{})
	release := make(chan struct{})
	var once sync.Once
	c := NewCache(SetMember(expired), SetCapture(func(string, interface{}) {
		once.Do(func() {
			close(firstDelete)
			<-release
		})
	}))
	defer c.Shutdown()

	done := make(chan struct{})
	go func() {
		_ = c.Iterator()
		close(done)
	}()
	select {
	case <-firstDelete:
	case <-time.After(time.Second):
		t.Fatal("Iterator did not begin expired-entry deletion")
	}

	for i := 0; i < keyCount; i++ {
		c.Set(fmt.Sprintf("k-%02d", i), "fresh", NoExpire)
	}
	close(release)
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("Iterator did not finish")
	}

	for i := 0; i < keyCount; i++ {
		key := fmt.Sprintf("k-%02d", i)
		if got, ok := c.Get(key); !ok || got != "fresh" {
			t.Fatalf("fresh replacement %q was deleted: got (%v, %v)", key, got, ok)
		}
	}
}

func TestCaptureConcurrentChangeAndDelete(t *testing.T) {
	c := NewCache(SetCapture(func(string, interface{}) {}))
	defer c.Shutdown()
	start := make(chan struct{})
	var wg sync.WaitGroup
	wg.Add(2)

	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100_000; i++ {
			c.ChangeCapture(func(string, interface{}) {})
		}
	}()
	go func() {
		defer wg.Done()
		<-start
		for i := 0; i < 100_000; i++ {
			c.Set("expired", i, time.Nanosecond)
			c.DeleteExpire()
			c.Set("deleted", i, NoExpire)
			c.Delete("deleted")
		}
	}()

	close(start)
	wg.Wait()
}
