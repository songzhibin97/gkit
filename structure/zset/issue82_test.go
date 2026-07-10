package zset

import "testing"

func TestDeleteNodePreservesNonEmptyTopLevel(t *testing.T) {
	list := newFloat64List()
	first := newFloat64ListNode(1, "first", 2)
	second := newFloat64ListNode(2, "second", 2)

	list.header.storeNextAndSpan(0, first, 1)
	list.header.storeNextAndSpan(1, first, 1)
	first.storeNextAndSpan(0, second, 1)
	first.storeNextAndSpan(1, second, 1)
	second.prev = first
	list.tail = second
	list.length = 2
	list.highestLevel = 2

	var update [maxLevel]*float64ListNode
	update[0] = list.header
	update[1] = list.header
	list.deleteNode(first, &update)

	if list.highestLevel != 2 {
		t.Fatalf("highestLevel after deleting first = %d, want 2", list.highestLevel)
	}
	if got := list.header.loadNext(1); got != second {
		t.Fatalf("top-level link = %p, want remaining node %p", got, second)
	}

	list.deleteNode(second, &update)
	if list.highestLevel != 1 {
		t.Fatalf("highestLevel after deleting final top node = %d, want 1", list.highestLevel)
	}
}

func TestDeleteNodeShrinksAllEmptyTopLevels(t *testing.T) {
	list := newFloat64List()
	only := newFloat64ListNode(1, "only", 4)
	var update [maxLevel]*float64ListNode
	for level := 0; level < 4; level++ {
		list.header.storeNextAndSpan(level, only, 1)
		update[level] = list.header
	}
	list.tail = only
	list.length = 1
	list.highestLevel = 4

	list.deleteNode(only, &update)

	if list.highestLevel != 1 {
		t.Fatalf("highestLevel after deleting only level-4 node = %d, want 1", list.highestLevel)
	}
	if list.length != 0 || list.tail != nil {
		t.Fatalf("list after final delete = length %d, tail %p; want empty", list.length, list.tail)
	}
	for level := 0; level < 4; level++ {
		if next, span := list.header.loadNextAndSpan(level); next != nil || span != 0 {
			t.Fatalf("header level %d after final delete = next %p, span %d; want nil, 0", level, next, span)
		}
	}
	if node := list.GetNodeByRank(1); node != nil {
		t.Fatalf("GetNodeByRank(1) after final delete = %q, want nil", node.value)
	}
}

func TestRangeBoundariesNeverExposeHeader(t *testing.T) {
	set := NewFloat64()
	set.Add(1, "a")
	set.Add(2, "b")
	set.Add(3, "c")

	assertValues := func(name string, got []Float64Node, want ...string) {
		t.Helper()
		if len(got) != len(want) {
			t.Fatalf("%s length = %d (%#v), want %d", name, len(got), got, len(want))
		}
		for index := range want {
			if got[index].Value != want[index] {
				t.Fatalf("%s[%d] = %q, want %q", name, index, got[index].Value, want[index])
			}
			if got[index].Value == "__HEADER" {
				t.Fatalf("%s exposed the header sentinel", name)
			}
		}
	}

	assertValues("Range(-4,-1)", set.Range(-4, -1), "a", "b", "c")
	assertValues("Range(0,-4)", set.Range(0, -4))
	assertValues("Range(0,99)", set.Range(0, 99), "a", "b", "c")
	assertValues("Range(3,3)", set.Range(3, 3))
	assertValues("RevRange(-4,-1)", set.RevRange(-4, -1), "c", "b", "a")
	assertValues("RevRange(0,-4)", set.RevRange(0, -4))
	assertValues("RevRange(3,3)", set.RevRange(3, 3))
	if node := set.list.GetNodeByRank(0); node != nil {
		t.Fatalf("GetNodeByRank(0) = %q, want nil", node.value)
	}
}
