package fileparse

import (
	"Songzhibin/GKit/cache/buffer"
	"errors"
	"fmt"
	"go/ast"
	"text/template"
)

// GoParsePB: .go 文件转成 pb文件
type GoParsePB struct {
	PackageName string
	Serves      []*Server
	Messages    []*Message
	Meta        map[string]string
}

// CreateGoParsePB: 创建 GoParsePB meta
func CreateGoParsePB(pkgName string) *GoParsePB {
	return &GoParsePB{
		PackageName: pkgName,
		Meta:        make(map[string]string),
	}
}

// AddServers: 添加server信息
func (g *GoParsePB) AddServers(servers ...*Server) {
	g.Serves = append(g.Serves, servers...)
}

// AddMessage: 添加message信息
func (g *GoParsePB) AddMessage(messages ...*Message) {
	g.Messages = append(g.Messages, messages...)
}

// Server: Server对应Go func
type Server struct {
	Name            string
	ServerName      string
	Method          string
	Router          string
	Doc             []string
	InputParameter  string
	OutputParameter string
}

// CreateServer: 创建Server
func CreateServer(name string, doc []string, inputParameter string, outputParameter string) *Server {
	return &Server{
		Name:            name,
		Doc:             doc,
		InputParameter:  inputParameter,
		OutputParameter: outputParameter,
	}
}

// Message: Message对应struct
type Message struct {
	Name  string
	Files []*File
}

// AddFiles: 添加字段信息
func (m *Message) AddFiles(files ...*File) {
	m.Files = append(m.Files, files...)
}

// CreateMessage: 创建Message
func CreateMessage(name string) *Message {
	return &Message{
		Name: name,
	}
}

type File struct {
	Tag          string
	Name         string
	TypeGo       string
	TypePB       string
	CustomizeTag []func(f *File)
}

func CreateFile(tag string, name string, tGo string, tPb string) *File {
	return &File{
		Tag:    tag,
		Name:   name,
		TypeGo: tGo,
		TypePB: tPb,
	}
}

// parseStruct: 解析struct信息
func (g *GoParsePB) parseStruct(st *ast.GenDecl, parseTag ...func(file *File)) {
	for _, spec := range st.Specs {
		if v, ok := spec.(*ast.TypeSpec); ok {
			ret := CreateMessage(v.Name.Name)
			if sType, ok := v.Type.(*ast.StructType); ok {

				for _, field := range sType.Fields.List {
					var (
						tag, name string
					)
					if field.Tag != nil {
						tag = field.Tag.Value
					}
					if field.Names != nil {
						name = field.Names[0].Name
					}

					if field.Type != nil {
						switch tType := field.Type.(type) {
						case *ast.InterfaceType:
							// 去除接口类型
							continue

						case *ast.Ident:
							if tType.Obj != nil {
								// 去除接口类型
								continue
							}
							tGo := fmt.Sprintf(`%s`, tType.Name)
							tPb := fmt.Sprintf("%s", GoTypeToPB(tType.Name))
							if name == "" {
								ret.AddFiles(CreateFile(tag, tPb, tGo, tPb))
							} else {
								ret.AddFiles(CreateFile(tag, name, tGo, tPb))
							}

						case *ast.ArrayType:
							if aType, ok := tType.Elt.(*ast.Ident); ok {
								tGo := fmt.Sprintf(`[]%s`, aType.Name)
								if aType.Name == "byte" {
									tPb := `bytes`
									ret.AddFiles(CreateFile(tag, name, tGo, tPb))
								} else {
									tPb := fmt.Sprintf(`repeated %s`, GoTypeToPB(aType.Name))
									ret.AddFiles(CreateFile(tag, name, tGo, tPb))
								}

							}
						case *ast.MapType:
							// 判断是否是 Ident
							mKey, ok := tType.Key.(*ast.Ident)
							if !ok {
								continue
							}
							mValue, ok := tType.Key.(*ast.Ident)
							if !ok {
								continue
							}
							if IsMappingKey(GoTypeToPB(mKey.Name)) {
								mk, mv := GoTypeToPB(mKey.Name), GoTypeToPB(mValue.Name)
								tGo := fmt.Sprintf(`map[%s]%s`, mk, mv)
								tPb := fmt.Sprintf(`map<%s,%s>`, mk, mv)
								ret.AddFiles(CreateFile(tag, name, tGo, tPb))
							}
						}
					}
				}

				// 执行tag解析
				for _, f := range parseTag {
					for _, file := range ret.Files {
						f(file)
					}
				}

				g.AddMessage(ret)
			}
		}
	}
}

// parseFunc: 解析函数信息
func (g *GoParsePB) parseFunc(fn *ast.FuncDecl, parseDocs ...func(*Server)) {
	var (
		tags            []string
		name            string
		inputParameter  string
		outputParameter string
	)
	if fn.Doc != nil {
		tags = make([]string, len(fn.Doc.List))
		for i, v := range fn.Doc.List {
			tags[i] = v.Text
		}
	}
	name = fn.Name.Name

	if fn.Type != nil {
		t := fn.Type
		if t.Params != nil && t.Params.List != nil {
			switch parameter := t.Params.List[0].Type.(type) {
			case *ast.Ident:
				inputParameter = parameter.Name
			}
		}
		if t.Results != nil && t.Results.List != nil {
			switch parameter := t.Results.List[0].Type.(type) {
			case *ast.Ident:
				outputParameter = parameter.Name
			}
		}
	}
	ret := CreateServer(name, tags, inputParameter, outputParameter)
	for _, f := range parseDocs {
		f(ret)
	}
	g.AddServers(ret)
}

func (g *GoParsePB) checkFormat() error {
	if _, ok := g.Meta["ServerName"]; ok {
		return nil
	}
	msgHashSet := make(map[string]struct{})
	// 去重
	for _, message := range g.Messages {
		if _, ok := msgHashSet[message.Name]; ok {
			return errors.New("message repeat")
		}
		msgHashSet[message.Name] = struct{}{}
	}
	serverHashSet := make(map[string]struct{})
	for _, serve := range g.Serves {
		if serve.ServerName != "" {
			g.Meta["ServerName"] = serve.ServerName
		}
		if serve.Router == "" || serve.Method == "" {
			return errors.New("server router or method is empty")
		}
		if _, ok := serverHashSet[serve.Router+serve.Method]; ok {
			return errors.New("server router method repeat")
		}
		if _, ok := msgHashSet[serve.InputParameter]; !ok {
			return errors.New("server input Parameters is empty")
		}
		if _, ok := msgHashSet[serve.OutputParameter]; !ok {
			return errors.New("server output Parameters is empty")
		}
		serverHashSet[serve.Router+serve.Method] = struct{}{}
	}
	return nil
}

func (g *GoParsePB) GeneratePB() string {
	if err := g.checkFormat(); err != nil {
		fmt.Println(err)
		return ""
	}
	var temp = `syntax = "proto3";
package {{.PackageName}};

// message{{range .Messages}}
message {{.Name}}{
{{range  $index, $messages :=.Files}}   {{$messages.TypePB}} {{$messages.Name}} = {{addOne $index}};
{{end}}}
{{end}}

// server
service {{.Meta.ServerName}}{
{{range .Serves}}  rpc {{.Name }} ({{.InputParameter}}) returns ({{.OutputParameter}}) {
    option (google.api.http) = {
      {{.Method}} : "{{.Router}}"
    };
  }
{{end}}}
`
	tmpl, err := template.New("GeneratePB").Funcs(template.FuncMap{"addOne": addOne}).Parse(temp)
	if err != nil {
		return ""
	}
	b := buffer.NewIoBuffer(1024)
	err = tmpl.Execute(b, g)
	if err != nil {
		return ""
	}
	return b.String()
}
