package parse_go

import (
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

// docTagValue returns the trimmed part after the first colon.
func docTagValue(doc string) (string, bool) {
	parts := strings.SplitN(doc, ":", 2)
	if len(parts) != 2 {
		return "", false
	}
	return strings.TrimSpace(parts[1]), true
}

func normalizeDocLine(doc string) string {
	doc = strings.TrimSpace(doc)
	doc = strings.TrimSpace(strings.TrimPrefix(doc, "//"))
	doc = strings.TrimSpace(strings.TrimPrefix(doc, "/*"))
	doc = strings.TrimSpace(strings.TrimSuffix(doc, "*/"))
	doc = strings.TrimSpace(strings.TrimPrefix(doc, "*"))
	return doc
}

func parseDoc(server *Server) {
	for _, doc := range server.Doc {
		for _, line := range strings.Split(doc, "\n") {
			line = normalizeDocLine(line)
			switch {
			case strings.HasPrefix(line, "@method:"):
				if v, ok := docTagValue(line); ok {
					server.Method = v
				}
			case strings.HasPrefix(line, "@service:"):
				if v, ok := docTagValue(line); ok {
					server.ServerName = v
				}
			case strings.HasPrefix(line, "@router:"):
				if v, ok := docTagValue(line); ok {
					server.Router = v
				}
			}
		}
	}
}
