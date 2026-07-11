package buffer

import (
	"bytes"
	"errors"
	"testing"
	"time"
)

const pipeZeroReadTimeout = 200 * time.Millisecond

func TestPipeZeroLengthReadReturnsImmediately(t *testing.T) {
	for _, test := range []struct {
		name string
		read []byte
	}{
		{name: "nil", read: nil},
		{name: "empty", read: []byte{}},
	} {
		t.Run(test.name, func(t *testing.T) {
			pipe := NewPipe(16)
			type result struct {
				n   int
				err error
			}
			done := make(chan result, 1)
			go func() {
				n, err := pipe.Read(test.read)
				done <- result{n: n, err: err}
			}()

			select {
			case got := <-done:
				if got.n != 0 || got.err != nil {
					t.Fatalf("Read(zero-length) = (%d, %v), want (0, nil)", got.n, got.err)
				}
			case <-time.After(pipeZeroReadTimeout):
				unblockErr := errors.New("unblock timed-out zero-length read")
				pipe.CloseWithError(unblockErr)
				<-done
				t.Fatalf("Read(zero-length) blocked for %s", pipeZeroReadTimeout)
			}

			if _, err := pipe.Write([]byte("x")); err != nil {
				t.Fatal(err)
			}
			read := make([]byte, 1)
			if n, err := pipe.Read(read); n != 1 || err != nil || string(read) != "x" {
				t.Fatalf("pipe after zero-length read = (%d, %v, %q)", n, err, read)
			}
			pipe.CloseWithError(nil)
		})
	}
}

func TestIoBufferPeekRejectsNegativeCountWithoutMutation(t *testing.T) {
	buf := NewIoBufferBytes([]byte("abcdef"))
	if got := buf.Peek(-1); got != nil {
		t.Fatalf("Peek(-1) = %q, want nil", got)
	}
	if got := buf.Peek(3); !bytes.Equal(got, []byte("abc")) {
		t.Fatalf("Peek(3) after Peek(-1) = %q, want abc", got)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte("abcdef")) {
		t.Fatalf("Peek(-1) mutated buffer: %q", got)
	}
}

func TestIoBufferDrainRejectsNegativeCountWithoutMutation(t *testing.T) {
	buf := NewIoBufferBytes([]byte("abcdef"))
	beforeLen := buf.Len()
	buf.Drain(-1)
	if got := buf.Len(); got != beforeLen {
		t.Fatalf("Len after Drain(-1) = %d, want %d", got, beforeLen)
	}
	buf.Drain(2)
	if got := buf.Bytes(); !bytes.Equal(got, []byte("cdef")) {
		t.Fatalf("Bytes after Drain(-1), Drain(2) = %q, want cdef", got)
	}
}

func TestIoBufferGrowRejectsNegativeCountWithoutMutation(t *testing.T) {
	buf := NewIoBufferBytes([]byte("abcdef"))
	err := buf.Grow(-1)
	if !errors.Is(err, ErrNegativeCount) {
		t.Errorf("Grow(-1) error = %v, want %v", err, ErrNegativeCount)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte("abcdef")) {
		t.Errorf("Grow(-1) mutated buffer: %q", got)
	}
	if err := buf.Grow(2); err != nil {
		t.Fatal(err)
	}
	if got := buf.Bytes(); !bytes.Equal(got, []byte{'a', 'b', 'c', 'd', 'e', 'f', 0, 0}) {
		t.Fatalf("Bytes after Grow(2) = %v", got)
	}
}
