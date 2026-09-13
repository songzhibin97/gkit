package deepcopy

import "testing"

func TestDeepCopyNilFieldsOverwriteDestination(t *testing.T) {
	type fields struct {
		P       *int
		M       map[string]int
		L       []int
		I       interface{}
		Typed   interface{}
		private *int
		Skip    *int `gkit:"-"`
		Scalar  int
	}
	n := 7
	src := fields{Typed: (*int)(nil)}
	dst := fields{P: &n, M: map[string]int{"old": 1}, L: []int{1}, I: 1, Typed: 1, private: &n, Skip: &n, Scalar: 3}
	if err := DeepCopy(&dst, &src); err != nil {
		t.Fatal(err)
	}
	if dst.P != nil || dst.M != nil || dst.L != nil || dst.I != nil || dst.private != nil || dst.Scalar != 0 {
		t.Fatalf("nil/zero fields not overwritten: %+v", dst)
	}
	if value, ok := dst.Typed.(*int); !ok || value != nil {
		t.Fatalf("typed nil = %#v", dst.Typed)
	}
	if dst.Skip != &n {
		t.Fatal("skipped field overwritten")
	}
	var p *int
	if err := DeepCopy(&dst.P, &p); err != nil || dst.P != nil {
		t.Fatalf("nil pointer root: %v, %v", dst.P, err)
	}
}
