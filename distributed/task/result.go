package task

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/bsontype"
)

// Result 任务返回携带的kv键值对
type Result struct {
	// Type 标注返回的类型
	Type string `json:"type" bson:"type"`
	// Value 根据type解压value
	Value interface{} `json:"value" bson:"value"`
}

// UnmarshalJSON restores supported result types without routing integers
// through float64. Native values also remain safe to persist through BSON.
// Unknown types retain their JSON shape with json.Number for numeric values.
func (r *Result) UnmarshalJSON(data []byte) error {
	var wire struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(data, &wire); err != nil {
		return err
	}
	var value interface{}
	if len(wire.Value) != 0 && !bytes.Equal(bytes.TrimSpace(wire.Value), []byte("null")) {
		if typ, ok := typeOfMap[wire.Type]; ok {
			target := reflect.New(typ)
			if err := json.Unmarshal(wire.Value, target.Interface()); err != nil {
				return fmt.Errorf("decode result type %q: %w", wire.Type, err)
			}
			value = target.Elem().Interface()
		} else {
			decoder := json.NewDecoder(bytes.NewReader(wire.Value))
			decoder.UseNumber()
			if err := decoder.Decode(&value); err != nil {
				return fmt.Errorf("decode result type %q: %w", wire.Type, err)
			}
		}
	}
	*r = Result{Type: wire.Type, Value: value}
	return nil
}

// UnmarshalBSON restores the declared result type after BSON turns unsigned
// integers into signed integers and byte slices into binary values. Old
// durable receipts can contain JSON's base64 strings for byte slices.
func (r *Result) UnmarshalBSON(data []byte) error {
	var wire struct {
		Type  string        `bson:"type"`
		Value bson.RawValue `bson:"value"`
	}
	if err := bson.Unmarshal(data, &wire); err != nil {
		return err
	}
	var value interface{}
	if wire.Value.Type != 0 && wire.Value.Type != bsontype.Null && wire.Value.Type != bsontype.Undefined {
		if wire.Type == "[]uint8" && wire.Value.Type == bsontype.String {
			decoded, err := base64.StdEncoding.DecodeString(wire.Value.StringValue())
			if err != nil {
				return fmt.Errorf("decode BSON result type %q: %w", wire.Type, err)
			}
			value = decoded
		} else if typ, ok := typeOfMap[wire.Type]; ok {
			target := reflect.New(typ)
			if err := wire.Value.Unmarshal(target.Interface()); err != nil {
				return fmt.Errorf("decode BSON result type %q: %w", wire.Type, err)
			}
			value = target.Elem().Interface()
		} else if err := wire.Value.Unmarshal(&value); err != nil {
			return fmt.Errorf("decode BSON result type %q: %w", wire.Type, err)
		}
	}
	*r = Result{Type: wire.Type, Value: value}
	return nil
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
