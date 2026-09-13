package tcp

import (
	"net"
	"sync"
	"testing"
	"time"
)

func TestDeadlineSettersConcurrent(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	c := NewConnByNetConn(a)
	var wg sync.WaitGroup
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for i := 0; i < 100; i++ {
				d := time.Now().Add(time.Hour)
				for _, set := range []func(time.Time) error{c.SetDeadline, c.SetReadDeadline, c.SetWriteDeadline, c.SetRecvDeadline, c.SetSendDeadline} {
					if err := set(d); err != nil {
						t.Error(err)
					}
				}
				c.SetRecvBufferInterval(time.Hour)
			}
		}()
	}
	wg.Wait()
}

type observedReadConn struct {
	net.Conn
	reads    int
	reading  chan struct{}
	mu       sync.Mutex
	deadline time.Time
}

func (c *observedReadConn) Read(p []byte) (int, error) {
	c.reads++
	if c.reads == 2 {
		close(c.reading)
	}
	return c.Conn.Read(p)
}
func (c *observedReadConn) SetReadDeadline(d time.Time) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if err := c.Conn.SetReadDeadline(d); err != nil {
		return err
	}
	c.deadline = d
	return nil
}

func TestStreamRestoreKeepsConcurrentDeadline(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	raw := &observedReadConn{Conn: a, reading: make(chan struct{})}
	c := NewConnByNetConn(raw)
	c.SetRecvBufferInterval(time.Hour)
	done := make(chan sendRecvResult, 1)
	go func() { data, err := c.Recv(-1, nil); done <- sendRecvResult{data, err} }()
	if _, err := b.Write([]byte("partial")); err != nil {
		t.Fatal(err)
	}
	<-raw.reading
	deadline := time.Now().Add(-time.Second)
	setDone := make(chan error, 1)
	go func() { setDone <- c.SetReadDeadline(deadline) }()
	select {
	case err := <-setDone:
		if err != nil {
			t.Fatal(err)
		}
	case <-time.After(time.Second):
		t.Fatal("deadline setter blocked behind Read")
	}
	select {
	case r := <-done:
		if string(r.data) != "partial" {
			t.Fatalf("data = %q", r.data)
		}
	case <-time.After(time.Second):
		t.Fatal("deadline did not wake Read")
	}
	raw.mu.Lock()
	actual := raw.deadline
	raw.mu.Unlock()
	if !actual.Equal(deadline) || !c.recvTimeout.Equal(deadline) {
		t.Fatalf("restoration overwrote external deadline: actual=%v local=%v want=%v", actual, c.recvTimeout, deadline)
	}
}

func TestDeadlineSetterWakesEmbeddedRead(t *testing.T) {
	a, b := net.Pipe()
	defer a.Close()
	defer b.Close()
	c := NewConnByNetConn(a)
	done := make(chan error, 1)
	go func() { _, err := c.Read(make([]byte, 1)); done <- err }()
	if err := c.SetDeadline(time.Now().Add(-time.Second)); err != nil {
		t.Fatal(err)
	}
	select {
	case err := <-done:
		if !isTimeout(err) {
			t.Fatalf("Read error = %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("Read did not wake")
	}
}
