package log

import (
	"bytes"
	"errors"
	"io"
	"testing"
)

type writerFunc func([]byte) (int, error)

func (f writerFunc) Write(p []byte) (int, error) {
	return f(p)
}

func TestStdLoggerPropagatesWriterError(t *testing.T) {
	want := errors.New("writer failed")
	logger := NewStdLogger(writerFunc(func(p []byte) (int, error) {
		return len(p) / 2, want
	}))

	err := logger.Log(LevelError, "key", "value")
	if !errors.Is(err, want) {
		t.Fatalf("Log error = %v, want underlying writer error %v", err, want)
	}
}

func TestStdLoggerReturnsShortWrite(t *testing.T) {
	logger := NewStdLogger(writerFunc(func(p []byte) (int, error) {
		return len(p) - 1, nil
	}))

	err := logger.Log(LevelInfo, "key", "value")
	if !errors.Is(err, io.ErrShortWrite) {
		t.Fatalf("Log error = %v, want %v", err, io.ErrShortWrite)
	}
}

func TestStdLoggerSuccessfulWriteUnchanged(t *testing.T) {
	var output bytes.Buffer
	logger := NewStdLogger(&output)

	if err := logger.Log(LevelWarn, "key", "value"); err != nil {
		t.Fatalf("Log returned error: %v", err)
	}
	if got, want := output.String(), "[Warn] key=value\n"; got != want {
		t.Fatalf("output = %q, want %q", got, want)
	}
}
