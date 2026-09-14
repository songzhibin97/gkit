package vto

import "testing"

func TestFieldBindPromotedNilPointer(t *testing.T) {
	type Inner struct{ Value int }
	type Middle struct{ *Inner }
	type Source struct {
		*Middle
		Direct int
	}
	type Target struct {
		Value  int `default:"13"`
		Direct int
	}
	binders := []struct {
		name string
		bind func(interface{}, interface{}) error
	}{
		{"basic", VoToDo},
		{"plus", func(d, s interface{}) error { return VoToDoPlus(d, s, ModelParameters{Model: FieldBind}) }},
	}
	for _, binder := range binders {
		for _, source := range []Source{{Middle: &Middle{Inner: &Inner{Value: 7}}, Direct: 5}, {Middle: &Middle{}, Direct: 5}, {Direct: 5}} {
			t.Run(binder.name, func(t *testing.T) {
				target := Target{Value: 42}
				if err := binder.bind(&target, &source); err != nil {
					t.Fatal(err)
				}
				want := 42
				if source.Middle != nil && source.Inner != nil {
					want = 7
				}
				if target.Value != want || target.Direct != 5 {
					t.Fatalf("target=%+v want Value=%d Direct=5", target, want)
				}
			})
		}
	}
}
