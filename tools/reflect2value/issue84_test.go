package reflect2value

import (
	"reflect"
	"testing"
)

func TestReflectValueSliceTypesRejectScalarAndAcceptSliceOrArray(t *testing.T) {
	tests := []struct {
		valueType string
		slice     interface{}
		array     interface{}
		want      interface{}
	}{
		{valueType: "[]bool", slice: []bool{true, false}, array: [2]bool{true, false}, want: []bool{true, false}},
		{valueType: "[]int", slice: []int64{1, 2}, array: [2]int64{1, 2}, want: []int{1, 2}},
		{valueType: "[]int8", slice: []int64{1, 2}, array: [2]int64{1, 2}, want: []int8{1, 2}},
		{valueType: "[]int16", slice: []int64{1, 2}, array: [2]int64{1, 2}, want: []int16{1, 2}},
		{valueType: "[]int32", slice: []int64{1, 2}, array: [2]int64{1, 2}, want: []int32{1, 2}},
		{valueType: "[]int64", slice: []int64{1, 2}, array: [2]int64{1, 2}, want: []int64{1, 2}},
		{valueType: "[]uint", slice: []uint64{1, 2}, array: [2]uint64{1, 2}, want: []uint{1, 2}},
		{valueType: "[]uint8", slice: []uint64{1, 2}, array: [2]uint64{1, 2}, want: []uint8{1, 2}},
		{valueType: "[]uint16", slice: []uint64{1, 2}, array: [2]uint64{1, 2}, want: []uint16{1, 2}},
		{valueType: "[]uint32", slice: []uint64{1, 2}, array: [2]uint64{1, 2}, want: []uint32{1, 2}},
		{valueType: "[]uint64", slice: []uint64{1, 2}, array: [2]uint64{1, 2}, want: []uint64{1, 2}},
		{valueType: "[]float32", slice: []float64{1.25, 2.5}, array: [2]float64{1.25, 2.5}, want: []float32{1.25, 2.5}},
		{valueType: "[]float64", slice: []float64{1.25, 2.5}, array: [2]float64{1.25, 2.5}, want: []float64{1.25, 2.5}},
		{valueType: "[]string", slice: []string{"one", "two"}, array: [2]string{"one", "two"}, want: []string{"one", "two"}},
	}

	for _, tt := range tests {
		t.Run(tt.valueType, func(t *testing.T) {
			if _, err := ReflectValue(tt.valueType, int64(1)); err == nil {
				t.Fatal("scalar input returned nil error")
			}
			for name, input := range map[string]interface{}{"slice": tt.slice, "array": tt.array} {
				t.Run(name, func(t *testing.T) {
					got, err := ReflectValue(tt.valueType, input)
					if err != nil {
						t.Fatalf("ReflectValue: %v", err)
					}
					if !reflect.DeepEqual(got.Interface(), tt.want) {
						t.Fatalf("ReflectValue = %#v, want %#v", got.Interface(), tt.want)
					}
				})
			}
		})
	}
}

func TestReflectValueUintSliceKeepsBase64Input(t *testing.T) {
	got, err := ReflectValue("[]uint8", "AQI=")
	if err != nil {
		t.Fatalf("ReflectValue: %v", err)
	}
	if !reflect.DeepEqual(got.Interface(), []uint8{1, 2}) {
		t.Fatalf("ReflectValue = %#v, want []uint8{1, 2}", got.Interface())
	}
}
