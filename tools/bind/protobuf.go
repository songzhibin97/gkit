package bind

import (
	"fmt"
	"io/ioutil"
	"net/http"
	"reflect"

	"google.golang.org/protobuf/proto"
)

type protobufBinding struct{}

func (protobufBinding) Name() string {
	return "protobuf"
}

func (b protobufBinding) Bind(req *http.Request, obj interface{}) error {
	buf, err := ioutil.ReadAll(req.Body)
	if err != nil {
		return err
	}
	return b.BindBody(buf, obj)
}

func (protobufBinding) BindBody(body []byte, obj interface{}) error {
	message, ok := obj.(proto.Message)
	if !ok {
		return fmt.Errorf("protobuf: target %T does not implement proto.Message", obj)
	}
	messageValue := reflect.ValueOf(message)
	if !messageValue.IsValid() || isNilProtobufMessage(messageValue) {
		return fmt.Errorf("protobuf: target %T is a nil proto.Message", obj)
	}
	if err := proto.Unmarshal(body, message); err != nil {
		return err
	}
	// Here it's same to return validate(obj), but util now we can't add
	// `binding:""` to the struct which automatically generate by gen-proto
	return nil
	// return validate(obj)
}

func isNilProtobufMessage(value reflect.Value) bool {
	switch value.Kind() {
	case reflect.Chan, reflect.Func, reflect.Interface, reflect.Map, reflect.Ptr, reflect.Slice:
		return value.IsNil()
	default:
		return false
	}
}
