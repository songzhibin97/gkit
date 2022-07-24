package task

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	jsoniter "github.com/json-iterator/go"
	"reflect"
	"strings"
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

		//"[]byte": reflect.TypeOf(make([]byte, 0)),
		//"[]rune": reflect.TypeOf(make([]rune, 0)),

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
	ctxTypeInterface = reflect.TypeOf((*context.Context)(nil)).Elem()

	retrievableInterface = reflect.TypeOf((*Retrievable)(nil)).Elem()

	errInterface = reflect.TypeOf((*error)(nil)).Elem()

	typeConversionError = func(argValue interface{}, argTypeStr string) error {
		return fmt.Errorf("%v is not %v", argValue, argTypeStr)
	}
)

// ReflectTaskResults ...
func ReflectTaskResults(taskResults []*Result) ([]reflect.Value, error) {
	resultValues := make([]reflect.Value, len(taskResults))
	for i, taskResult := range taskResults {
		resultValue, err := ReflectValue(taskResult.Type, taskResult.Value)
		if err != nil {
			return nil, err
		}
		resultValues[i] = resultValue
	}
	return resultValues, nil
}

// HumanReadableResults ...
func HumanReadableResults(results []reflect.Value) string {
	if len(results) == 1 {
		return fmt.Sprintf("%v", results[0].Interface())
	}

	readableResults := make([]string, len(results))
	for i := 0; i < len(results); i++ {
		readableResults[i] = fmt.Sprintf("%v", results[i].Interface())
	}

	return fmt.Sprintf("[%s]", strings.Join(readableResults, ", "))
}

// ReflectValue converts interface{} to reflect.Value based on string type
func ReflectValue(valueType string, value interface{}) (reflect.Value, error) {
	if strings.HasPrefix(valueType, "[]") {
		return reflectValues(valueType, value)
	}

	return reflectValue(valueType, value)
}

// reflectValue converts interface{} to reflect.Value based on string type
// representing a base type (not a slice)
func reflectValue(valueType string, value interface{}) (reflect.Value, error) {
	theType, ok := typeOfMap[valueType]
	if !ok {
		return reflect.Value{}, NewErrNonsupportType(valueType)
	}
	theValue := reflect.New(theType)

	// Booleans
	if theType.String() == "bool" {
		boolValue, err := getBoolValue(theType.String(), value)
		if err != nil {
			return reflect.Value{}, err
		}

		theValue.Elem().SetBool(boolValue)
		return theValue.Elem(), nil
	}

	// Integers
	if strings.HasPrefix(theType.String(), "int") {
		intValue, err := getIntValue(theType.String(), value)
		if err != nil {
			return reflect.Value{}, err
		}

		theValue.Elem().SetInt(intValue)
		return theValue.Elem(), err
	}

	// Unsigned integers
	if strings.HasPrefix(theType.String(), "uint") {
		uintValue, err := getUintValue(theType.String(), value)
		if err != nil {
			return reflect.Value{}, err
		}

		theValue.Elem().SetUint(uintValue)
		return theValue.Elem(), err
	}

	// Floating point numbers
	if strings.HasPrefix(theType.String(), "float") {
		floatValue, err := getFloatValue(theType.String(), value)
		if err != nil {
			return reflect.Value{}, err
		}

		theValue.Elem().SetFloat(floatValue)
		return theValue.Elem(), err
	}

	// Strings
	if theType.String() == "string" {
		stringValue, err := getStringValue(theType.String(), value)
		if err != nil {
			return reflect.Value{}, err
		}

		theValue.Elem().SetString(stringValue)
		return theValue.Elem(), nil
	}

	return reflect.Value{}, NewErrNonsupportType(valueType)
}

// reflectValues converts interface{} to reflect.Value based on string type
// representing a slice of values
func reflectValues(valueType string, value interface{}) (reflect.Value, error) {
	theType, ok := typeOfMap[valueType]
	if !ok {
		return reflect.Value{}, NewErrNonsupportType(valueType)
	}

	// For NULL we return an empty slice
	if value == nil {
		return reflect.MakeSlice(theType, 0, 0), nil
	}

	var theValue reflect.Value

	// Booleans
	if theType.String() == "[]bool" {
		bools := reflect.ValueOf(value)

		theValue = reflect.MakeSlice(theType, bools.Len(), bools.Len())
		for i := 0; i < bools.Len(); i++ {
			boolValue, err := getBoolValue(strings.Split(theType.String(), "[]")[1], bools.Index(i).Interface())
			if err != nil {
				return reflect.Value{}, err
			}

			theValue.Index(i).SetBool(boolValue)
		}

		return theValue, nil
	}

	// Integers
	if strings.HasPrefix(theType.String(), "[]int") {
		ints := reflect.ValueOf(value)

		theValue = reflect.MakeSlice(theType, ints.Len(), ints.Len())
		for i := 0; i < ints.Len(); i++ {
			intValue, err := getIntValue(strings.Split(theType.String(), "[]")[1], ints.Index(i).Interface())
			if err != nil {
				return reflect.Value{}, err
			}

			theValue.Index(i).SetInt(intValue)
		}

		return theValue, nil
	}

	// Unsigned integers
	if strings.HasPrefix(theType.String(), "[]uint") || theType.String() == "[]byte" {

		// Decode the base64 string if the value type is []uint8 or it's alias []byte
		// See: https://golang.org/pkg/encoding/json/#Marshal
		// > Array and slice values encode as JSON arrays, except that []byte encodes as a base64-encoded string
		if reflect.TypeOf(value).String() == "string" {
			output, err := base64.StdEncoding.DecodeString(value.(string))
			if err != nil {
				return reflect.Value{}, err
			}
			value = output
		}

		uints := reflect.ValueOf(value)

		theValue = reflect.MakeSlice(theType, uints.Len(), uints.Len())
		for i := 0; i < uints.Len(); i++ {
			uintValue, err := getUintValue(strings.Split(theType.String(), "[]")[1], uints.Index(i).Interface())
			if err != nil {
				return reflect.Value{}, err
			}

			theValue.Index(i).SetUint(uintValue)
		}

		return theValue, nil
	}

	// Floating point numbers
	if strings.HasPrefix(theType.String(), "[]float") {
		floats := reflect.ValueOf(value)

		theValue = reflect.MakeSlice(theType, floats.Len(), floats.Len())
		for i := 0; i < floats.Len(); i++ {
			floatValue, err := getFloatValue(strings.Split(theType.String(), "[]")[1], floats.Index(i).Interface())
			if err != nil {
				return reflect.Value{}, err
			}

			theValue.Index(i).SetFloat(floatValue)
		}

		return theValue, nil
	}

	// Strings
	if theType.String() == "[]string" {
		strs := reflect.ValueOf(value)

		theValue = reflect.MakeSlice(theType, strs.Len(), strs.Len())
		for i := 0; i < strs.Len(); i++ {
			strValue, err := getStringValue(strings.Split(theType.String(), "[]")[1], strs.Index(i).Interface())
			if err != nil {
				return reflect.Value{}, err
			}

			theValue.Index(i).SetString(strValue)
		}

		return theValue, nil
	}

	return reflect.Value{}, NewErrNonsupportType(valueType)
}

func getBoolValue(theType string, value interface{}) (bool, error) {
	b, ok := value.(bool)
	if !ok {
		return false, typeConversionError(value, typeOfMap[theType].String())
	}

	return b, nil
}

func getIntValue(theType string, value interface{}) (int64, error) {
	// We use https://golang.org/pkg/encoding/json/#Decoder.UseNumber when unmarshaling signatures.
	// This is because JSON only supports 64-bit floating point numbers and we could lose precision
	// when converting from float64 to signed integer
	if strings.HasPrefix(fmt.Sprintf("%T", value), "json.Number") || strings.HasPrefix(fmt.Sprintf("%T", value), "jsoniter.Number") {
		switch n := value.(type) {
		case json.Number:
			return n.Int64()
		case jsoniter.Number:
			return n.Int64()
		default:
			return 0, typeConversionError(value, typeOfMap[theType].String())
		}
	}

	var n int64
	switch value := value.(type) {
	case int64:
		n = value
	case int32:
		n = int64(value)
	case int16:
		n = int64(value)
	case int8:
		n = int64(value)
	case int:
		n = int64(value)
	default:
		return 0, typeConversionError(value, typeOfMap[theType].String())
	}
	return n, nil
}

func getUintValue(theType string, value interface{}) (uint64, error) {
	// We use https://golang.org/pkg/encoding/json/#Decoder.UseNumber when unmarshaling signatures.
	// This is because JSON only supports 64-bit floating point numbers and we could lose precision
	// when converting from float64 to unsigned integer
	if strings.HasPrefix(fmt.Sprintf("%T", value), "json.Number") || strings.HasPrefix(fmt.Sprintf("%T", value), "jsoniter.Number") {
		switch n := value.(type) {
		case json.Number:
			intVal, err := n.Int64()
			if err != nil {
				return 0, err
			}

			return uint64(intVal), nil
		case jsoniter.Number:
			intVal, err := n.Int64()
			if err != nil {
				return 0, err
			}

			return uint64(intVal), nil
		default:
			return 0, typeConversionError(value, typeOfMap[theType].String())
		}
	}

	var n uint64
	switch value := value.(type) {
	case uint64:
		n = value
	case uint32:
		n = uint64(value)
	case uint16:
		n = uint64(value)
	case uint8:
		n = uint64(value)
	case uint:
		n = uint64(value)
	default:
		return 0, typeConversionError(value, typeOfMap[theType].String())
	}
	return n, nil
}

func getFloatValue(theType string, value interface{}) (float64, error) {
	// We use https://golang.org/pkg/encoding/json/#Decoder.UseNumber when unmarshaling signatures.
	// This is because JSON only supports 64-bit floating point numbers and we could lose precision
	if strings.HasPrefix(fmt.Sprintf("%T", value), "json.Number") || strings.HasPrefix(fmt.Sprintf("%T", value), "jsoniter.Number") {
		switch n := value.(type) {
		case json.Number:
			return n.Float64()
		case jsoniter.Number:
			return n.Float64()
		default:
			return 0, typeConversionError(value, typeOfMap[theType].String())
		}
	}

	var n float64
	switch value := value.(type) {
	case float64:
		n = value
	case float32:
		n = float64(value)
	default:
		return 0, typeConversionError(value, typeOfMap[theType].String())
	}
	return n, nil
}

func getStringValue(theType string, value interface{}) (string, error) {
	s, ok := value.(string)
	if !ok {
		return "", typeConversionError(value, typeOfMap[theType].String())
	}

	return s, nil
}
