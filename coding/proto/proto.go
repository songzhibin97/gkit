package proto

import (
	"fmt"
	"reflect"

	"github.com/songzhibin97/gkit/coding"
	"google.golang.org/protobuf/proto"
)

const Name = "proto"

func init() {
	_ = coding.RegisterCode(code{})
}

type code struct{}

func (c code) Marshal(v interface{}) ([]byte, error) {
	m, ok := v.(proto.Message)
	if !ok {
		return nil, fmt.Errorf("proto: Marshal expects proto.Message, got %T", v)
	}
	return proto.Marshal(m)
}

func (c code) Unmarshal(data []byte, v interface{}) error {
	m, ok := v.(proto.Message)
	if !ok {
		return fmt.Errorf("proto: Unmarshal expects proto.Message, got %T", v)
	}
	if rv := reflect.ValueOf(m); rv.Kind() == reflect.Ptr && rv.IsNil() {
		return fmt.Errorf("proto: unmarshal target is a nil %T", v)
	}
	return proto.Unmarshal(data, m)
}

func (c code) Name() string {
	return Name
}
