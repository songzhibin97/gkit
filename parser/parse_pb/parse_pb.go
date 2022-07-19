package parse_pb

import (
	"bytes"
	"io/ioutil"

	"github.com/emicklei/proto"
	"github.com/songzhibin97/gkit/options"
	"github.com/songzhibin97/gkit/parser"
)

func ParsePb(filepath string, options ...options.Option) (parser.Parser, error) {
	reader, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	parser := proto.NewParser(bytes.NewReader(reader))
	definition, err := parser.Parse()
	if err != nil {
		return nil, err
	}

	ret := CreatePbParseGo()
	for _, v := range options {
		v(ret)
	}
	ret.FilePath = filepath
	for _, element := range definition.Elements {
		switch v := element.(type) {
		case *proto.Package:
			ret.PkgName = v.Name
		case *proto.Comment:
			// note
			ret.AddNode(&Note{Comment: v})
		case *proto.Message:
			// message
			ret.parseMessage(v, "")
		case *proto.Service:
			// service
			ret.parseService(v)
		case *proto.Enum:
			ret.parseEnum(v, "")
		}
	}
	return ret, nil
}
