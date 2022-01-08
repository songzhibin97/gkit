package task

import (
	"reflect"
)

var (
	typeOfMap = map[string]reflect.Type{
		// basic
		"bool": reflect.TypeOf(false),

		"int":   reflect.TypeOf(int(0)),
		"int8":  reflect.TypeOf(int8(0)),
		"int16": reflect.TypeOf(int16(0)),
		"int32": reflect.TypeOf(int32(0)),
		"int64": reflect.TypeOf(int64(0)),

		"uint":   reflect.TypeOf(uint(0)),
		"uint8":  reflect.TypeOf(uint8(0)),
		"uint16": reflect.TypeOf(uint16(0)),
		"uint32": reflect.TypeOf(uint32(0)),
		"uint64": reflect.TypeOf(uint64(0)),

		"float32": reflect.TypeOf(float32(0)),
		"float64": reflect.TypeOf(float64(0)),

		"string": reflect.TypeOf(string("")),

		// compound

		// slice
		"[]bool": reflect.TypeOf(make([]bool, 0)),

		"[]byte": reflect.TypeOf(make([]byte, 0)),
		"[]rune": reflect.TypeOf(make([]rune, 0)),

		"[]int":   reflect.TypeOf(make([]int, 0)),
		"[]int8":  reflect.TypeOf(make([]int8, 0)),
		"[]int16": reflect.TypeOf(make([]int16, 0)),
		"[]int32": reflect.TypeOf(make([]int32, 0)),
		"[]int64": reflect.TypeOf(make([]int64, 0)),

		"[]uint":   reflect.TypeOf(make([]uint, 0)),
		"[]uint8":  reflect.TypeOf(make([]uint8, 0)),
		"[]uint16": reflect.TypeOf(make([]uint16, 0)),
		"[]uint32": reflect.TypeOf(make([]uint32, 0)),
		"[]uint64": reflect.TypeOf(make([]uint64, 0)),

		"[]float32": reflect.TypeOf(make([]float32, 0)),
		"[]float64": reflect.TypeOf(make([]float64, 0)),

		"[]string": reflect.TypeOf(make([]string, 0)),
	}
)

func ReflectValue(valueType string, value interface{}) (reflect.Value, error) {
	typeOf, ok := typeOfMap[valueType]
	if !ok {
		return reflect.Value{}, NewErrNonsupportType(valueType)
	}
	valueOf := reflect.New(typeOf)
	valueOf.Elem().Set(reflect.ValueOf(value))
	return valueOf.Elem(), nil
}
