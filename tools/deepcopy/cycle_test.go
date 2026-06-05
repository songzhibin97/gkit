package deepcopy

import (
	"reflect"
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

// TestClone_SelfReferentialSliceDoesNotStackOverflow covers the slice cycle the
// original fix missed: only Ptr/Map were registered in visited, so a slice that
// contains itself recursed forever and stack-overflowed.
func TestClone_SelfReferentialSliceDoesNotStackOverflow(t *testing.T) {
	s := make([]interface{}, 1)
	s[0] = s // self-reference

	done := make(chan interface{}, 1)
	go func() {
		done <- Clone(s)
	}()
	select {
	case got := <-done:
		cp, ok := got.([]interface{})
		if !ok {
			t.Fatalf("Clone returned %T, want []interface{}", got)
		}
		if len(cp) != 1 {
			t.Fatalf("len = %d, want 1", len(cp))
		}
		inner, ok := cp[0].([]interface{})
		if !ok {
			t.Fatalf("cp[0] = %T, want []interface{}", cp[0])
		}
		// The cycle must point back to the copy, not the original or a fresh slice.
		if reflect.ValueOf(inner).Pointer() != reflect.ValueOf(cp).Pointer() {
			t.Fatal("slice cycle not preserved: cp[0] is not cp")
		}
	case <-time.After(time.Second):
		t.Fatal("Clone of self-referential slice hung (regression to stack overflow)")
	}
}

// TestDeepCopy_AliasedSubSliceKeepsLength guards against keying slice cycles by
// backing-array address alone: a slice and a sub-slice that share a backing
// array (full and full[:3]) start at the same address but have different
// lengths, so they must be copied independently — not conflated into one copy
// of the wrong length.
func TestDeepCopy_AliasedSubSliceKeepsLength(t *testing.T) {
	type holder struct {
		Full []int
		Half []int
	}
	full := []int{0, 1, 2, 3, 4, 5}
	src := &holder{Full: full, Half: full[:3]}

	var dst holder
	if err := DeepCopy(&dst, src); err != nil {
		t.Fatalf("DeepCopy: %v", err)
	}
	if len(dst.Full) != 6 {
		t.Fatalf("Full len = %d, want 6", len(dst.Full))
	}
	if len(dst.Half) != 3 {
		t.Fatalf("Half len = %d, want 3 (sub-slice length must not be conflated with Full)", len(dst.Half))
	}
}

// TestDeepCopy_EmptySlicesDifferentTypesNoCollision guards the element type in
// the visit key: distinct zero-size allocations (make([]int,0), make([]string,0))
// can share a backing address, so keying without the type conflates them and
// dst.Set the wrong element type — a panic. Both must copy cleanly.
func TestDeepCopy_EmptySlicesDifferentTypesNoCollision(t *testing.T) {
	type holder struct {
		A []int
		B []string
	}
	src := &holder{A: make([]int, 0), B: make([]string, 0)}

	var dst holder
	if err := DeepCopy(&dst, src); err != nil {
		t.Fatalf("DeepCopy: %v", err)
	}
	if dst.A == nil || dst.B == nil {
		t.Fatalf("empty slices became nil: A=%v B=%v", dst.A, dst.B)
	}
	if len(dst.A) != 0 || len(dst.B) != 0 {
		t.Fatalf("lengths: A=%d B=%d, want 0,0", len(dst.A), len(dst.B))
	}
}
