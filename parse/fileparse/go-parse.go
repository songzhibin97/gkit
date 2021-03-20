package fileparse

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
)

func ParseGo(filepath string) (*GoParsePB, error) {
	data, err := ioutil.ReadFile(filepath)
	if err != nil {
		return nil, err
	}
	fSet := token.NewFileSet()
	// fParse: 解析到的内容
	fParse, err := parser.ParseFile(fSet, filepath, data, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	// 根据内容找到 struct 以及 func
	ret := CreateGoParsePB(fParse.Name.Name)
	for _, decl := range fParse.Decls {
		switch v := decl.(type) {
		case *ast.GenDecl:
			ret.parseStruct(v, parseTag)
		case *ast.FuncDecl:
			ret.parseFunc(v, parseDoc)
		}
	}
	return ret, nil
}

func parseTag(file *File) {
	tags := strings.Split(file.Tag, " ")
	for _, tag := range tags {
		if strings.Contains(tag, "gkit:") {
			num := strings.Split(strings.Replace(tag, "gkit:", "", -1), ",")
			for _, flag := range num {
				if strings.Contains(flag, "pType=") {
					t := []byte(strings.Replace(flag, "pType=", "", -1))
					r := make([]byte, 0, 10)
					for _, v := range t {
						if v == '"' || v == '`' || v == ';' {
							continue
						}
						r = append(r, v)
					}
					file.TypePB = string(r)
				}
			}
		}
	}
}

func parseDoc(server *Server) {
	for _, doc := range server.Doc {
		if strings.Contains(doc, "@method") {
			server.Method = strings.Split(doc, ":")[1]
		}
		if strings.Contains(doc, "@service") {
			server.ServerName = strings.Split(doc, ":")[1]
		}
		if strings.Contains(doc, "@router") {
			server.Router = strings.Split(doc, ":")[1]
		}
	}
}