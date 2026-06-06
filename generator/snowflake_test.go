package generator

import (
	"testing"
	"time"
)

func TestNewSnowflakeE_FutureStartTimeReturnsError(t *testing.T) {
	future := time.Now().Add(time.Hour)
	g, err := NewSnowflakeE(future, 1)
	if err != ErrStartTimeInFuture {
		t.Fatalf("err = %v, want ErrStartTimeInFuture", err)
	}
	if g != nil {
		t.Fatalf("g should be nil on error, got %T", g)
	}
}

func TestNewSnowflakeE_HappyPath(t *testing.T) {
	g, err := NewSnowflakeE(time.Time{}, 42)
	if err != nil {
		t.Fatalf("err = %v", err)
	}
	id, err := g.NextID()
	if err != nil {
		t.Fatalf("NextID: %v", err)
	}
	if id == 0 {
		t.Fatalf("ID = 0")
	}
}

func TestNewSnowflake_DeprecatedReturnsNil(t *testing.T) {
	// Verifies the deprecated path keeps its original (broken) behaviour for
	// the sake of API compatibility — callers that supplied a future start
	// time still get nil back.
	future := time.Now().Add(time.Hour)
	if g := NewSnowflake(future, 1); g != nil {
		t.Fatalf("deprecated NewSnowflake should return nil for future startTime, got %T", g)
	}
}
