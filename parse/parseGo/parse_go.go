package parseGo

import (
	"Songzhibin/GKit/parse"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"io/ioutil"
	"strings"
)

func ParseGo(filepath string) (parse.Parse, error) {
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
	comment := []*parse.Note{}
	for _, group := range fParse.Comments {
		for _, c := range group.List {
			comment = append(comment, &parse.Note{Comment: c})
		}
	}
	// 根据内容找到 struct 以及 func
	rets := CreateGoParsePB(fParse.Name.Name, filepath, comment)
	ret := rets.(*GoParsePB)
	for _, decl := range fParse.Decls {
		switch v := decl.(type) {
		case *ast.GenDecl:
			ret.parseStruct(v, parseTag)
		case *ast.FuncDecl:
			ret.parseFunc(v, parseDoc)
		}
	}
	return ret, ret.checkFormat()
}

func parseTag(file *parse.File) {
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

func parseDoc(server *parse.Server) {
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
