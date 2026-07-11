package task

import (
	stdjson "encoding/json"
	"reflect"
	"testing"

	json "github.com/json-iterator/go"
)

var reflectValuesTestCases = []struct {
	name          string
	value         interface{}
	expectedType  string
	expectedValue interface{}
}{
	// basic types
	{
		name:         "bool",
		value:        false,
		expectedType: "bool",
	},
	{
		name:          "int",
		value:         json.Number("123"),
		expectedType:  "int",
		expectedValue: int(123),
	},
	{
		name:          "int8",
		value:         json.Number("123"),
		expectedType:  "int8",
		expectedValue: int8(123),
	},
	{
		name:          "int16",
		value:         json.Number("123"),
		expectedType:  "int16",
		expectedValue: int16(123),
	},
	{
		name:          "int32",
		value:         json.Number("123"),
		expectedType:  "int32",
		expectedValue: int32(123),
	},
	{
		name:          "int64",
		value:         json.Number("185135722552891243"),
		expectedType:  "int64",
		expectedValue: int64(185135722552891243),
	},
	{
		name:          "uint",
		value:         json.Number("123"),
		expectedType:  "uint",
		expectedValue: uint(123),
	},
	{
		name:          "uint8",
		value:         json.Number("123"),
		expectedType:  "uint8",
		expectedValue: uint8(123),
	},
	{
		name:          "uint16",
		value:         json.Number("123"),
		expectedType:  "uint16",
		expectedValue: uint16(123),
	},
	{
		name:          "uint32",
		value:         json.Number("123"),
		expectedType:  "uint32",
		expectedValue: uint32(123),
	},
	{
		name:          "uint64",
		value:         json.Number("185135722552891243"),
		expectedType:  "uint64",
		expectedValue: uint64(185135722552891243),
	},
	{
		name:          "float32",
		value:         json.Number("0.5"),
		expectedType:  "float32",
		expectedValue: float32(0.5),
	},
	{
		name:          "float64",
		value:         json.Number("0.5"),
		expectedType:  "float64",
		expectedValue: float64(0.5),
	},
	{
		name:          "string",
		value:         "123",
		expectedType:  "string",
		expectedValue: "123",
	},
	// slices
	{
		name:          "[]bool",
		value:         []interface{}{false, true},
		expectedType:  "[]bool",
		expectedValue: []bool{false, true},
	},
	{
		name:          "[]int",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]int",
		expectedValue: []int{1, 2},
	},
	{
		name:          "[]int8",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]int8",
		expectedValue: []int8{1, 2},
	},
	{
		name:          "[]int16",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]int16",
		expectedValue: []int16{1, 2},
	},
	{
		name:          "[]int32",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]int32",
		expectedValue: []int32{1, 2},
	},
	{
		name:          "[]int64",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]int64",
		expectedValue: []int64{1, 2},
	},
	{
		name:          "[]uint",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]uint",
		expectedValue: []uint{1, 2},
	},
	{
		name:          "[]uint8",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]uint8",
		expectedValue: []uint8{1, 2},
	},
	{
		name:          "[]uint16",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]uint16",
		expectedValue: []uint16{1, 2},
	},
	{
		name:          "[]uint32",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]uint32",
		expectedValue: []uint32{1, 2},
	},
	{
		name:          "[]uint64",
		value:         []interface{}{json.Number("1"), json.Number("2")},
		expectedType:  "[]uint64",
		expectedValue: []uint64{1, 2},
	},
	{
		name:          "[]float32",
		value:         []interface{}{json.Number("0.5"), json.Number("1.28")},
		expectedType:  "[]float32",
		expectedValue: []float32{0.5, 1.28},
	},
	{
		name:          "[]float64",
		value:         []interface{}{json.Number("0.5"), json.Number("1.28")},
		expectedType:  "[]float64",
		expectedValue: []float64{0.5, 1.28},
	},
	{
		name:          "[]string",
		value:         []interface{}{"foo", "bar"},
		expectedType:  "[]string",
		expectedValue: []string{"foo", "bar"},
	},
	// empty slices from NULL
	{
		name:          "[]bool",
		value:         nil,
		expectedType:  "[]bool",
		expectedValue: []bool{},
	},
	{
		name:          "[]int64",
		value:         nil,
		expectedType:  "[]int64",
		expectedValue: []int64{},
	},
	{
		name:          "[]uint64",
		value:         nil,
		expectedType:  "[]uint64",
		expectedValue: []uint64{},
	},
	{
		name:          "[]float64",
		value:         nil,
		expectedType:  "[]float64",
		expectedValue: []float64{},
	},
	{
		name:          "[]string",
		value:         nil,
		expectedType:  "[]string",
		expectedValue: []string{},
	},
}

func TestReflectValue(t *testing.T) {
	t.Parallel()

	for _, tc := range reflectValuesTestCases {
		tc := tc // capture range variable
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			value, err := ReflectValue(tc.name, tc.value)
			if err != nil {
				t.Error(err)
			}
			if value.Type().String() != tc.expectedType {
				t.Errorf("type is %v, want %s", value.Type().String(), tc.expectedType)
			}
			if tc.expectedValue != nil {
				if !reflect.DeepEqual(value.Interface(), tc.expectedValue) {
					t.Errorf("value is %v, want %v", value.Interface(), tc.expectedValue)
				}
			}
		})
	}
}

func TestNewTaskWithSignatureRejectsNonSliceArgument(t *testing.T) {
	testCases := []struct {
		name      string
		value     interface{}
		wantError string
	}{
		{name: "int", value: 1, wantError: "1 is not []int"},
		{name: "bool", value: true, wantError: "true is not []int"},
		{name: "map", value: map[string]int{"value": 1}, wantError: "map[value:1] is not []int"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			defer func() {
				if recovered := recover(); recovered != nil {
					t.Fatalf("NewTaskWithSignature panicked for %T input: %v", tc.value, recovered)
				}
			}()

			signature := &Signature{Args: []Arg{{Type: "[]int", Value: tc.value}}}
			_, err := NewTaskWithSignature(func([]int) error { return nil }, signature)
			if err == nil {
				t.Fatal("NewTaskWithSignature returned nil error for non-slice argument")
			}
			if err.Error() != tc.wantError {
				t.Fatalf("NewTaskWithSignature error = %q, want %q", err, tc.wantError)
			}
		})
	}
}

func TestReflectValueAcceptsSliceAndArrayInput(t *testing.T) {
	testCases := []struct {
		name  string
		value interface{}
	}{
		{name: "slice", value: []interface{}{json.Number("1"), json.Number("2")}},
		{name: "array", value: [2]interface{}{json.Number("1"), json.Number("2")}},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			value, err := ReflectValue("[]int", tc.value)
			if err != nil {
				t.Fatalf("ReflectValue returned error: %v", err)
			}
			if got, want := value.Interface(), []int{1, 2}; !reflect.DeepEqual(got, want) {
				t.Fatalf("ReflectValue value = %v, want %v", got, want)
			}
		})
	}
}

func TestReflectValuePreservesJSONNumberError(t *testing.T) {
	number := stdjson.Number("not-a-number")
	_, wantErr := number.Int64()

	_, err := ReflectValue("[]int", []interface{}{number})
	if err == nil {
		t.Fatal("ReflectValue returned nil error for invalid json.Number")
	}
	if err.Error() != wantErr.Error() {
		t.Fatalf("ReflectValue error = %q, want %q", err, wantErr)
	}
}
