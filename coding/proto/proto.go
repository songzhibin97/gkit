package proto

import (
	"fmt"

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
	return proto.Unmarshal(data, m)
}

func (c code) Name() string {
	return Name
}
