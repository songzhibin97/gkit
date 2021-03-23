package parsePb

import (
	"Songzhibin/GKit/parse"
	"bytes"
	"fmt"
	"github.com/emicklei/proto"
	"io/ioutil"
)

func ParsePb(filepath string) (parse.Parse, error) {
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
	for _, element := range definition.Elements {
		switch v := element.(type) {
		case *proto.Package:
			ret.PkgName = v.Name
		case *proto.Comment:
			// note
			ret.Note = append(ret.Note, &Note{Comment: v})
		case *proto.Message:
			// message
			ret.parseMessage(v)
		case *proto.Service:
			// service
			ret.parseService(v)

		}
	}
	return nil, err
}

func handleService(s *proto.Service) {
	fmt.Println(s.Name)
}

func handleMessage(m *proto.Message) {
	fmt.Println(m.Name)
}
