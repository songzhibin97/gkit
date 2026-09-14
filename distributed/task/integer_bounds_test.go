package task

import (
	"encoding/json"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"testing"

	jsoniter "github.com/json-iterator/go"
)

// Regression for #166 / 03-02: integer labels must not silently narrow values.
func TestReflectIntegerJSONBoundaries(t *testing.T) {
	tests := []struct{ kind, min, max, below, above string }{
		{"int8", "-128", "127", "-129", "128"},
		{"int16", "-32768", "32767", "-32769", "32768"},
		{"int32", "-2147483648", "2147483647", "-2147483649", "2147483648"},
		{"int64", "-9223372036854775808", "9223372036854775807", "-9223372036854775809", "9223372036854775808"},
		{"uint8", "0", "255", "-1", "256"},
		{"uint16", "0", "65535", "-1", "65536"},
		{"uint32", "0", "4294967295", "-1", "4294967296"},
		{"uint64", "0", "18446744073709551615", "-1", "18446744073709551616"},
	}
	intIndex, uintIndex := 3, 7
	if strconv.IntSize == 32 {
		intIndex, uintIndex = 2, 6
	}
	signed, unsigned := tests[intIndex], tests[uintIndex]
	signed.kind, unsigned.kind = "int", "uint"
	tests = append(tests, signed, unsigned)
	for _, test := range tests {
		for _, format := range []string{"std", "jsoniter"} {
			for _, slice := range []bool{false, true} {
				kind := test.kind
				if slice {
					kind = "[]" + kind
				}
				t.Run(kind+"/"+format, func(t *testing.T) {
					for _, number := range []string{test.min, "0", test.max, test.below, test.above} {
						var input interface{} = json.Number(number)
						if format == "jsoniter" {
							input = jsoniter.Number(number)
						}
						if slice {
							input = []interface{}{json.Number("0"), input}
						}
						got, err := ReflectValue(kind, input)
						if number == test.below || number == test.above {
							if err == nil || got.IsValid() {
								t.Errorf("%s(%s) = (%v, %v), want invalid value and range error", kind, number, got, err)
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
							if got.Len() != 2 || fmt.Sprint(got.Index(0).Interface()) != "0" {
								t.Fatalf("slice = %v, want leading zero and boundary", got.Interface())
							}
							got = got.Index(1)
						}
						if fmt.Sprint(got.Interface()) != number {
							t.Errorf("%s(%s) = %v", kind, number, got.Interface())
						}
					}
				})
			}
		}
	}
}

func TestReflectIntegerNativeNarrowing(t *testing.T) {
	for _, test := range []struct {
		kind        string
		input, good interface{}
		want        string
	}{
		{"int8", int64(128), int64(127), "127"}, {"int8", int16(-129), int16(-128), "-128"},
		{"int16", int32(32768), int32(32767), "32767"}, {"int32", int64(2147483648), int64(2147483647), "2147483647"},
		{"uint8", uint64(256), uint64(255), "255"}, {"uint16", uint32(65536), uint32(65535), "65535"},
		{"uint32", uint64(4294967296), uint64(4294967295), "4294967295"},
	} {
		for _, slice := range []bool{false, true} {
			kind, input, good := test.kind, test.input, test.good
			if slice {
				kind, input = "[]"+kind, []interface{}{input}
				good = []interface{}{good}
			}
			t.Run(fmt.Sprintf("%s/%v", kind, test.input), func(t *testing.T) {
				if got, err := ReflectValue(kind, input); err == nil || got.IsValid() {
					t.Fatalf("narrowing = (%v, %v), want error", got, err)
				}
				got, err := ReflectValue(kind, good)
				if err != nil {
					t.Fatalf("valid native boundary: %v", err)
				}
				if slice {
					if got.Len() != 1 {
						t.Fatalf("slice length = %d, want 1", got.Len())
					}
					got = got.Index(0)
				}
				if fmt.Sprint(got.Interface()) != test.want {
					t.Fatalf("valid native boundary = %v, want %s", got.Interface(), test.want)
				}
			})
		}
	}
}

func TestTaskIntegerArgsPreserveValidBoundaries(t *testing.T) {
	called := false
	fn := func(signed int8, unsigned uint64, values []uint8) error {
		called = true
		if signed != 127 || unsigned != math.MaxUint64 || !reflect.DeepEqual(values, []uint8{0, 255}) {
			t.Errorf("handler received (%d, %d, %v)", signed, unsigned, values)
		}
		return nil
	}
	signature := &Signature{Args: []Arg{
		{Type: "int8", Value: json.Number("127")},
		{Type: "uint64", Value: jsoniter.Number("18446744073709551615")},
		{Type: "[]uint8", Value: []interface{}{json.Number("0"), jsoniter.Number("255")}},
	}}
	exec, err := NewTaskWithSignature(fn, signature)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := exec.Call(); err != nil {
		t.Fatal(err)
	}
	if !called {
		t.Fatal("handler did not execute")
	}
	signature.Args[0].Value = json.Number("128")
	if exec, err := NewTaskWithSignature(fn, signature); err == nil || exec != nil {
		t.Fatalf("out-of-range task constructed: %v, %v", exec, err)
	}
}
