package bind

import "testing"

func TestUriEmptyScalarValues(t *testing.T) {
	for _, pointer := range []bool{false, true} {
		for _, values := range [][]string{nil, {}, {""}, {"first", "last"}} {
			target := map[string]string{"value": "old", "untouched": "keep"}
			var obj interface{} = target
			if pointer {
				obj = &target
			}
			if err := Uri.BindUri(map[string][]string{"value": values}, obj); err != nil {
				t.Fatal(err)
			}
			want := ""
			if len(values) > 0 {
				want = values[len(values)-1]
			}
			if got, ok := target["value"]; !ok || got != want || target["untouched"] != "keep" {
				t.Fatalf("pointer=%v values=%#v target=%#v want=%q", pointer, values, target, want)
			}
		}
	}
	var allocated map[string]string
	if err := Uri.BindUri(map[string][]string{"value": nil}, &allocated); err != nil {
		t.Fatal(err)
	}
	if got, ok := allocated["value"]; !ok || got != "" {
		t.Fatalf("allocated map = %#v", allocated)
	}
}
