package buffer

import (
	"bytes"
	"errors"
	"io"
	"runtime"
	"testing"
	"time"
)

func TestIoBufferReadEmptyReturnsEOF(t *testing.T) {
	tests := []struct {
		name  string
		setup func(IoBuffer)
	}{
		{name: "new"},
		{
			name: "reset",
			setup: func(buf IoBuffer) {
				_, _ = buf.Write([]byte{1})
				buf.Reset()
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewIoBuffer(16)
			defer PutIoPool(buf)
			if tt.setup != nil {
				tt.setup(buf)
			}

			n, err := buf.Read(make([]byte, 1))
			if n != 0 || !errors.Is(err, io.EOF) {
				t.Fatalf("Read(empty) = (%d, %v), want (0, io.EOF)", n, err)
			}
		})
	}
}

func TestIoBufferZeroLengthReadReturnsNil(t *testing.T) {
	tests := []struct {
		name string
		data []byte
	}{
		{name: "empty"},
		{name: "non-empty", data: []byte{1}},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			buf := NewIoBuffer(16)
			defer PutIoPool(buf)
			_, _ = buf.Write(tt.data)

			n, err := buf.Read(nil)
			if n != 0 || err != nil {
				t.Fatalf("Read(zero-length) = (%d, %v), want (0, nil)", n, err)
			}
			if got := buf.Bytes(); !bytes.Equal(got, tt.data) {
				t.Fatalf("Read(zero-length) consumed data: got %v, want %v", got, tt.data)
			}
		})
	}
}

func blockedPipeReaders() int {
	stack := make([]byte, 1<<20)
	n := runtime.Stack(stack, true)
	return bytes.Count(stack[:n], []byte("cache/buffer.(*pipe).Read"))
}

func waitForBlockedPipeReaders(t *testing.T, want int) {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		if blockedPipeReaders() >= want {
			return
		}
		runtime.Gosched()
		time.Sleep(time.Millisecond)
	}
	t.Fatalf("blocked pipe readers = %d, want at least %d", blockedPipeReaders(), want)
}

func TestPipeCloseWakesAllBlockedReaders(t *testing.T) {
	pipe := NewPipe(16)
	started := make(chan struct{}, 2)
	done := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() {
			started <- struct{}{}
			_, err := pipe.Read(make([]byte, 1))
			done <- err
		}()
	}
	<-started
	<-started
	waitForBlockedPipeReaders(t, 2)

	pipe.CloseWithError(nil)
	timer := time.NewTimer(300 * time.Millisecond)
	defer timer.Stop()
	for returned := 0; returned < 2; returned++ {
		select {
		case err := <-done:
			if !errors.Is(err, io.EOF) {
				t.Fatalf("Read after CloseWithError(nil) returned %v, want io.EOF", err)
			}
		case <-timer.C:
			t.Fatalf("CloseWithError woke %d/2 blocked readers", returned)
		}
	}
}

func TestIoBufferGrowZeroInitializesNewRegion(t *testing.T) {
	backing := bytes.Repeat([]byte{0xa5}, 16)
	buf := NewIoBufferBytes(backing[:4])

	if err := buf.Grow(4); err != nil {
		t.Fatal(err)
	}
	want := []byte{0xa5, 0xa5, 0xa5, 0xa5, 0, 0, 0, 0}
	if got := buf.Bytes(); !bytes.Equal(got, want) {
		t.Fatalf("Bytes() after Grow = %x, want %x", got, want)
	}
}
