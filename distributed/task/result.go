package task

import (
	"fmt"
	"reflect"
	"strings"
)

// Result 任务返回携带的kv键值对
type Result struct {
	// Type 标注返回的类型
	Type string `json:"type" bson:"type"`
	// Value 根据type解压value
	Value interface{} `json:"value" bson:"value"`
}

// ConvertResult 将Result类型转换成reflect.Value
func ConvertResult(result []*Result) ([]reflect.Value, error) {
	convertResult := make([]reflect.Value, 0, len(result))
	for _, r := range result {
		_value, err := ReflectValue(r.Type, r.Value)
		if err != nil {
			return nil, err
		}
		convertResult = append(convertResult, _value)
	}
	return convertResult, nil
}

// FormatResult 将reflect.Value转换为可读答案
func FormatResult(values []reflect.Value) string {
	ln := len(values)
	switch ln {
	case 0:
		return ""
	case 1:
		return fmt.Sprintf("[%v]", values[0].Interface())
	default:
		builder := strings.Builder{}
		for i, value := range values {
			if i == 0 {
				builder.WriteString("[ ")
			}
			builder.WriteString(fmt.Sprintf("%v", value.Interface()))
			if i != ln-1 {
				builder.WriteString(", ")
			} else {
				builder.WriteString(" ]")
			}
		}
		return builder.String()
	}
}
