//go:build !nomsgpack
// +build !nomsgpack

package bind

import (
	"bytes"
	"io"
	"net/http"

	"github.com/ugorji/go/codec"
)

type msgpackBinding struct{}

func (msgpackBinding) Name() string {
	return "msgpack"
}

func (msgpackBinding) Bind(req *http.Request, obj interface{}) error {
	return decodeMsgPack(req.Body, obj)
}

func (msgpackBinding) BindBody(body []byte, obj interface{}) error {
	return decodeMsgPack(bytes.NewReader(body), obj)
}

func decodeMsgPack(r io.Reader, obj interface{}) error {
	cdc := new(codec.MsgpackHandle)
	return codec.NewDecoder(r, cdc).Decode(&obj)
}
