package zset

import (
	"math"
	"reflect"
	"testing"
)

func TestRankOfMemberMatchingHeader(t *testing.T) {
	z := NewFloat64()
	first := newFloat64ListNode(-math.MaxFloat64, "__HEADER", 1)
	second := newFloat64ListNode(1, "second", 2)
	// These heights can occur through Add. Fix them here so the search starts
	// above the real member, without relying on random height selection.
	z.list.header.storeNextAndSpan(0, first, 1)
	z.list.header.storeNextAndSpan(1, second, 2)
	first.storeNextAndSpan(0, second, 1)
	second.prev = first
	z.list.tail = second
	z.list.length = 2
	z.list.highestLevel = 2
	z.dict[first.value] = first.score
	z.dict[second.value] = second.score

	if first == z.list.header || first.level != 1 || second.level != 2 || first.prev != nil || second.prev != first || z.list.tail != second {
		t.Fatal("invalid member heights or backward links")
	}
	if z.list.header.loadNext(0) != first || z.list.header.loadSpan(0) != 1 || z.list.header.loadNext(1) != second || z.list.header.loadSpan(1) != 2 || first.loadNext(0) != second || first.loadSpan(0) != 1 {
		t.Fatal("invalid forward links or spans")
	}
	for level := 0; level < second.level; level++ {
		if second.loadNext(level) != nil || second.loadSpan(level) != 0 {
			t.Fatal("invalid tail link or span")
		}
	}
	want := []Float64Node{{Value: "__HEADER", Score: -math.MaxFloat64}, {Value: "second", Score: 1}}
	if got := z.Range(0, -1); !reflect.DeepEqual(got, want) {
		t.Fatalf("Range = %#v, want %#v", got, want)
	}
	if got := z.RevRange(0, -1); !reflect.DeepEqual(got, []Float64Node{want[1], want[0]}) {
		t.Fatalf("RevRange = %#v", got)
	}
	assertFloat64SetConsistent(t, z)
	for i, n := range want {
		if got := z.Rank(n.Value); got != i {
			t.Errorf("Rank(%q) = %d, want %d", n.Value, got, i)
		}
		if got := z.RevRank(n.Value); got != len(want)-1-i {
			t.Errorf("RevRank(%q) = %d, want %d", n.Value, got, len(want)-1-i)
		}
	}
	if got := z.Count(-math.MaxFloat64, 1); got != 2 {
		t.Errorf("Count = %d, want 2", got)
	}
	if got := z.CountWithOpt(-math.MaxFloat64, 1, RangeOpt{ExcludeMax: true}); got != 1 {
		t.Errorf("Count excluding max = %d, want 1", got)
	}
	if got := z.CountWithOpt(-math.MaxFloat64, 1, RangeOpt{ExcludeMin: true}); got != 1 {
		t.Errorf("Count excluding min = %d, want 1", got)
	}
	if z.Rank("missing") != -1 || z.RevRank("missing") != -1 {
		t.Error("missing member has a rank")
	}
}
