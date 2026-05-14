package deepcopy

import (
	"testing"
	"time"
)

type cyclicNode struct {
	Name string
	Next *cyclicNode
}

// TestClone_SelfReferentialDoesNotStackOverflow covers C25: previously a
// self-referencing pointer (`n.Next = n`) recursed forever in deepCopy
// and crashed with stack overflow.
func TestClone_SelfReferentialDoesNotStackOverflow(t *testing.T) {
	n := &cyclicNode{Name: "root"}
	n.Next = n

	done := make(chan interface{}, 1)
	go func() {
		done <- Clone(n)
	}()
	select {
	case got := <-done:
		c, ok := got.(*cyclicNode)
		if !ok {
			t.Fatalf("Clone returned %T, want *cyclicNode", got)
		}
		if c.Name != "root" {
			t.Fatalf("Name = %q", c.Name)
		}
		// The cycle must be preserved: c.Next should refer back to c (the
		// copy), not the original.
		if c.Next != c {
			t.Fatalf("cycle not preserved: c.Next != c")
		}
	case <-time.After(time.Second):
		t.Fatal("Clone of self-referential struct hung (regression to stack overflow)")
	}
}
