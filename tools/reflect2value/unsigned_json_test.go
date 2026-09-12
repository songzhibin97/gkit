package reflect2value

import (
	"encoding/json"
	"fmt"
	"strconv"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

// Regression for #166 / 03-02: parse unsigned JSON in its full unsigned range.
func TestReflectUnsignedJSONRange(t *testing.T) {
	tests := []struct{ kind, max, above string }{
		{"uint8", "255", "256"}, {"uint16", "65535", "65536"},
		{"uint32", "4294967295", "4294967296"},
		{"uint64", "18446744073709551615", "18446744073709551616"},
	}
	index := 3
	if strconv.IntSize == 32 {
		index = 2
	}
	native := tests[index]
	native.kind = "uint"
	tests = append(tests, native)
	for _, test := range tests {
		for _, format := range []string{"std", "jsoniter"} {
			for _, slice := range []bool{false, true} {
				kind := test.kind
				if slice {
					kind = "[]" + kind
				}
				t.Run(kind+"/"+format, func(t *testing.T) {
					for _, number := range []string{"0", test.max, "-1", test.above} {
						var input interface{} = json.Number(number)
						if format == "jsoniter" {
							input = jsoniter.Number(number)
						}
						if slice {
							input = []interface{}{json.Number("0"), input}
						}
						got, err := ReflectValue(kind, input)
						if number == "-1" || number == test.above {
							if err == nil || got.IsValid() {
								t.Errorf("%s(%s) = (%v, %v), want range error", kind, number, got, err)
							}
							continue
						}
						if err != nil {
							t.Errorf("%s(%s): %v", kind, number, err)
							continue
						}
						if got.Type().String() != kind {
							t.Fatalf("type = %s, want %s", got.Type(), kind)
						}
						if slice {
							if got.Len() != 2 || got.Index(0).Uint() != 0 {
								t.Fatalf("unexpected slice: %v", got.Interface())
							}
							got = got.Index(1)
						}
						if fmt.Sprint(got.Interface()) != number {
							t.Errorf("value = %v, want %s", got.Interface(), number)
						}
					}
				})
			}
		}
	}
}
