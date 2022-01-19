package json

import (
	"reflect"

	json "github.com/json-iterator/go"

	"github.com/songzhibin97/gkit/coding"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

const Name = "json"

var (
	MarshalOptions = protojson.MarshalOptions{
		EmitUnpopulated: true,
		UseProtoNames:   true,
	}

	UnmarshalOptions = protojson.UnmarshalOptions{
		DiscardUnknown: true,
	}
)

func init() {
	_ = coding.RegisterCode(code{})
}

type code struct{}

func (c code) Marshal(v interface{}) ([]byte, error) {
	if m, ok := v.(proto.Message); ok {
		return MarshalOptions.Marshal(m)
	}
	return json.Marshal(v)
}

func (c code) Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}
	if m, ok := v.(proto.Message); ok {
		return UnmarshalOptions.Unmarshal(data, m)
	} else if m, ok := reflect.Indirect(reflect.ValueOf(v)).Interface().(proto.Message); ok {
		return UnmarshalOptions.Unmarshal(data, m)
	}
	return json.Unmarshal(data, v)
}

func (c code) Name() string {
	return Name
}
