package proto

import (
	"github.com/songzhibin97/gkit/coding"
	"google.golang.org/protobuf/proto"
)

const Name = "proto"

func init() {
	_ = coding.RegisterCode(code{})
}

type code struct{}

func (c code) Marshal(v interface{}) ([]byte, error) {
	return proto.Marshal(v.(proto.Message))
}

func (c code) Unmarshal(data []byte, v interface{}) error {
	return proto.Unmarshal(data, v.(proto.Message))
}

func (c code) Name() string {
	return Name
}
