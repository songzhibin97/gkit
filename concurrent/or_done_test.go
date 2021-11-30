package concurrent

import (
	"testing"
	"time"
)

func signal(d time.Duration) <-chan interface{} {
	c := make(chan interface{}, 1)
	go func() {
		defer close(c)
		time.Sleep(d)
	}()
	return c
}

func TestOrDone(t *testing.T) {
	start := time.Now()
	<-OrDone(
		signal(10*time.Second),
		signal(20*time.Second),
		signal(30*time.Second),
		signal(40*time.Second),
		signal(50*time.Second),
		signal(3*time.Second),
	)
	t.Log("done", time.Since(start))
}
