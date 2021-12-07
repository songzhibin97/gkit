package yaml

import (
	"reflect"

	"github.com/songzhibin97/gkit/coding"
	"gopkg.in/yaml.v2"
)

const Name = "yaml"

func init() {
	_ = coding.RegisterCode(code{})
}

type code struct{}

func (c code) Marshal(v interface{}) ([]byte, error) {
	return yaml.Marshal(v)
}

func (c code) Unmarshal(data []byte, v interface{}) error {
	rv := reflect.ValueOf(v)
	for rv.Kind() == reflect.Ptr {
		if rv.IsNil() {
			rv.Set(reflect.New(rv.Type().Elem()))
		}
		rv = rv.Elem()
	}
	return yaml.Unmarshal(data, v)
}

func (c code) Name() string {
	return Name
}
