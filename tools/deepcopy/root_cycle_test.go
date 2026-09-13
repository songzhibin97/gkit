package deepcopy

import "testing"

func TestDeepCopyPreservesRootBackReferences(t *testing.T) {
	type node struct {
		Value    int
		Next     *node
		Back     interface{}
		Children map[string]*node
	}
	for _, indirect := range []bool{false, true} {
		src := &node{Value: 7}
		src.Next = src
		if indirect {
			src.Next = &node{Value: 8, Next: src}
		}
		src.Back = src
		src.Children = map[string]*node{"root": src}
		dst := &node{}
		if err := DeepCopy(dst, src); err != nil {
			t.Fatal(err)
		}
		back := dst.Next
		if indirect {
			if back == src.Next || back.Value != 8 {
				t.Fatal("child not copied")
			}
			back = back.Next
		}
		if back != dst || dst.Back != dst || dst.Children["root"] != dst {
			t.Fatalf("indirect=%v: root references do not reach destination", indirect)
		}
		dst.Value = 9
		if back.Value != 9 || src.Value != 7 {
			t.Fatal("root mutation topology or isolation lost")
		}
		clone := Clone(src).(*node)
		if clone.Back != clone || clone.Children["root"] != clone {
			t.Fatal("Clone control lost root references")
		}
	}
}
