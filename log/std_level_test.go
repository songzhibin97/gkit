package log

import (
	"bytes"
	"strings"
	"testing"
)

// TestStdLogger_LevelFilter covers C28: previously stdLogger.Log
// ignored the level parameter — `Lever.Allow` was defined but never
// called, so every level emitted regardless of the configured minimum.
func TestStdLogger_LevelFilter(t *testing.T) {
	var buf bytes.Buffer
	l := NewStdLoggerWithLevel(&buf, LevelWarn)

	_ = l.Log(LevelDebug, "k", "v")
	_ = l.Log(LevelInfo, "k", "v")
	if buf.Len() != 0 {
		t.Fatalf("Debug/Info emitted under Warn floor: %q", buf.String())
	}

	_ = l.Log(LevelWarn, "k", "v")
	if !strings.Contains(buf.String(), "Warn") {
		t.Fatalf("Warn not emitted: %q", buf.String())
	}
	_ = l.Log(LevelError, "k", "v")
	if !strings.Contains(buf.String(), "Error") {
		t.Fatalf("Error not emitted: %q", buf.String())
	}
}
