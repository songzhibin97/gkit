package parse_go

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"

	"github.com/songzhibin97/gkit/options"
	gparser "github.com/songzhibin97/gkit/parser"
)

func ParseGo(filepath string, options ...options.Option) (gparser.Parser, error) {
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
	// 将注释存档
	var comment []*Note
	for _, group := range fParse.Comments {
		for _, c := range group.List {
			comment = append(comment, &Note{Comment: c})
		}
	}
	// 根据内容找到 struct 以及 func
	ret := CreateGoParsePB(fParse.Name.Name, filepath, comment)
	for _, v := range options {
		v(ret)
	}
	for _, decl := range fParse.Decls {
		switch v := decl.(type) {
		case *ast.GenDecl:
			ret.parseStruct(v)
		case *ast.FuncDecl:
			ret.parseFunc(v)
		}
	}
	return ret, ret.checkFormat()
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
	defer func() {
		if err := recover(); err != nil {
			fmt.Println("panic:", err)
			return
		}
	}()
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
