package zset

import (
	"math"
	"testing"
)

func TestUnionFloat64SumsOverlappingScores(t *testing.T) {
	left := NewFloat64()
	left.Add(1.25, "common")
	left.Add(10, "left")
	right := NewFloat64()
	right.Add(2.75, "common")
	right.Add(20, "right")

	got := UnionFloat64(left, right)
	if score, ok := got.Score("common"); !ok || score != 4 {
		t.Fatalf("union common score = (%v, %v), want (4, true)", score, ok)
	}
	if got.Len() != 3 {
		t.Fatalf("union length = %d, want 3", got.Len())
	}
}

func TestInterFloat64SumsOverlappingScores(t *testing.T) {
	left := NewFloat64()
	left.Add(1.25, "common")
	left.Add(10, "left")
	right := NewFloat64()
	right.Add(2.75, "common")
	right.Add(20, "right")

	got := InterFloat64(left, right)
	if score, ok := got.Score("common"); !ok || score != 4 {
		t.Fatalf("intersection common score = (%v, %v), want (4, true)", score, ok)
	}
	if got.Len() != 1 {
		t.Fatalf("intersection length = %d, want 1", got.Len())
	}
}

func TestFloat64SetRejectsNaNWithoutMutation(t *testing.T) {
	z := NewFloat64()
	z.Add(1, "stable")

	if added := z.Add(math.NaN(), "new"); added {
		t.Fatal("Add(NaN) reported a newly-created member")
	}
	if z.Contains("new") {
		t.Fatal("Add(NaN) inserted a member")
	}
	if added := z.Add(math.NaN(), "stable"); added {
		t.Fatal("Add(NaN) reported an existing member as newly created")
	}
	if score, ok := z.Score("stable"); !ok || score != 1 {
		t.Fatalf("stable score after Add(NaN) = (%v, %v), want (1, true)", score, ok)
	}
	assertFloat64SetConsistent(t, z)
}

func TestFloat64SetIncrByRejectsNaNWithoutMutation(t *testing.T) {
	z := NewFloat64()
	z.Add(math.Inf(1), "infinite")

	if score, existed := z.IncrBy(math.NaN(), "new"); existed || score != 0 {
		t.Fatalf("IncrBy(NaN) for new member = (%v, %v), want (0, false)", score, existed)
	}
	if z.Contains("new") {
		t.Fatal("IncrBy(NaN) inserted a new member")
	}

	score, existed := z.IncrBy(math.Inf(-1), "infinite")
	if !existed || !math.IsInf(score, 1) {
		t.Fatalf("IncrBy(-Inf) after +Inf = (%v, %v), want (+Inf, true)", score, existed)
	}
	if stored, ok := z.Score("infinite"); !ok || !math.IsInf(stored, 1) {
		t.Fatalf("stored score after rejected NaN result = (%v, %v), want (+Inf, true)", stored, ok)
	}
	assertFloat64SetConsistent(t, z)
}

func TestSetOperationsRejectNaNAggregates(t *testing.T) {
	left := NewFloat64()
	left.Add(math.Inf(1), "nan-sum")
	left.Add(1, "left")
	right := NewFloat64()
	right.Add(math.Inf(-1), "nan-sum")
	right.Add(2, "right")

	union := UnionFloat64(left, right)
	if union.Contains("nan-sum") {
		t.Fatal("union retained a member whose aggregate score is NaN")
	}
	if union.Len() != 2 || !union.Contains("left") || !union.Contains("right") {
		t.Fatalf("union members = %v, want only left and right", union.Range(0, -1))
	}
	assertFloat64SetConsistent(t, union)

	intersection := InterFloat64(left, right)
	if intersection.Contains("nan-sum") || intersection.Len() != 0 {
		t.Fatalf("intersection members = %v, want empty after rejecting NaN aggregate", intersection.Range(0, -1))
	}
	assertFloat64SetConsistent(t, intersection)
}

func assertFloat64SetConsistent(t *testing.T, z *Float64Set) {
	t.Helper()
	nodes := z.Range(0, -1)
	if len(z.dict) != z.Len() {
		t.Fatalf("dict length = %d, Len = %d", len(z.dict), z.Len())
	}
	if len(nodes) != z.Len() {
		t.Fatalf("range length = %d, Len = %d", len(nodes), z.Len())
	}
	for _, node := range nodes {
		if math.IsNaN(node.Score) {
			t.Fatalf("skiplist contains NaN score for %q", node.Value)
		}
		score, ok := z.Score(node.Value)
		if !ok || score != node.Score {
			t.Fatalf("dict score for %q = (%v, %v), skiplist score = %v", node.Value, score, ok, node.Score)
		}
	}
}
