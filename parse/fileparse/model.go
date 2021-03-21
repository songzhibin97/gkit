package fileparse

import (
	"Songzhibin/GKit/cache/buffer"
	"errors"
	"fmt"
	"go/ast"
	"text/template"
)

// goParsePB: .go 文件转成 pb文件
type goParsePB struct {
	pkgName  string            // pkgName: 包名
	servers  []*Server         // servers: 解析出来function的信息
	messages []*Message        // messages: 解析出struct的信息
	notes    []*Note           // notes: 其他注释
	meta     map[string]string // meta: 其他元信息
}

type Note struct {
	IsUse bool // 判断作用域, 如果是 struct中 或者 func中代表已经使用
	*ast.Comment
}

// CreateGoParsePB: 创建 goParsePB meta
func CreateGoParsePB(pkgName string, notes []*Note) *goParsePB {
	return &goParsePB{
		pkgName: pkgName,
		meta:    make(map[string]string),
		notes:   notes,
	}
}

// Server: Server对应Go func
type Server struct {
	Pos             int            // Pos: 函数的起始字节位置
	End             int            // End: 函数的结束字节为止
	Name            string         // Name: 函数名
	ServerName      string         // ServerName: server name 通过 parseFunc 绑定
	Method          string         // Method: method 通过 parseFunc 绑定
	Router          string         // Router: router 通过 parseFunc 绑定
	InputParameter  string         // InputParameter: 入参
	OutputParameter string         // OutputParameter: 出参
	Doc             []string       // Doc: 函数注释信息,可以通过自定义的 parseFunc 去进行解析
	Notes           []*ast.Comment // Notes: 函数中的注释信息,用于埋点打桩

}

// CreateServer: 创建Server
func CreateServer(name string, pos, end int, doc []string, inputParameter string, outputParameter string) *Server {
	return &Server{
		Pos:             pos,
		End:             end,
		Name:            name,
		Doc:             doc,
		InputParameter:  inputParameter,
		OutputParameter: outputParameter,
	}
}

// Message: Message对应struct
type Message struct {
	Pos   int            // Pos: struct的起始字节位置
	End   int            // End: struct的结束字节为止
	Name  string         // Name: struct name
	Files []*File        // Files: 字段信息
	Notes []*ast.Comment // Notes: struct的注释信息,用于埋点打桩
}

// AddFiles: 添加字段信息
func (m *Message) AddFiles(files ...*File) {
	m.Files = append(m.Files, files...)
}

// CreateMessage: 创建Message
func CreateMessage(name string, pos, end int) *Message {
	return &Message{
		Name: name,
		Pos:  pos,
		End:  end,
	}
}

// File: 字段信息
type File struct {
	Tag    string // Tag: 字段的tag标记
	Name   string // Name: 字段名
	TypeGo string // TypeGo: 字段的原始类型
	TypePB string // TypePB: 字段在proto中的类型
}

// CreateFile: 创建字段信息
func CreateFile(tag string, name string, tGo string, tPb string) *File {
	return &File{
		Tag:    tag,
		Name:   name,
		TypeGo: tGo,
		TypePB: tPb,
	}
}

// parseStruct: 解析struct信息
func (g *goParsePB) parseStruct(st *ast.GenDecl, parseTag ...func(file *File)) {
	for _, spec := range st.Specs {
		if v, ok := spec.(*ast.TypeSpec); ok {
			ret := CreateMessage(v.Name.Name, int(v.Pos()), int(v.End()))
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

				g.AddMessages(ret)
			}
		}
	}
}

// parseFunc: 解析函数信息
func (g *goParsePB) parseFunc(fn *ast.FuncDecl, parseDocs ...func(*Server)) {
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
	ret := CreateServer(name, int(fn.Pos()), int(fn.End()), tags, inputParameter, outputParameter)
	for _, f := range parseDocs {
		f(ret)
	}
	g.AddServers(ret)
}

// checkFormat: 查重,以及确认服务中的出参入参是否在上文中出现
func (g *goParsePB) checkFormat() error {
	if _, ok := g.meta["ServerName"]; ok {
		return nil
	}
	msgHashSet := make(map[string]struct{})

	for _, message := range g.messages {
		if _, ok := msgHashSet[message.Name]; ok {
			return errors.New("message repeat")
		}
		for _, note := range g.notes {
			if !note.IsUse && int(note.Pos()) > message.Pos && int(note.End()) <= message.End {
				message.Notes = append(message.Notes, note.Comment)
				note.IsUse = true
			}

		}
		msgHashSet[message.Name] = struct{}{}
	}
	serverHashSet := make(map[string]struct{})
	for _, serve := range g.servers {
		if serve.ServerName != "" {
			g.meta["ServerName"] = serve.ServerName
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
		for _, note := range g.notes {
			if !note.IsUse && int(note.Pos()) > serve.Pos && int(note.End()) <= serve.End {
				serve.Notes = append(serve.Notes, note.Comment)
				note.IsUse = true
			}
		}
		serverHashSet[serve.Router+serve.Method] = struct{}{}
	}
	return nil
}

// Servers: 返回解析后的所有Server对象
func (g *goParsePB) Servers() []*Server {
	return g.servers
}

// Messages: 返回解析后的所有Message对象
func (g *goParsePB) Messages() []*Message {
	return g.messages
}

// AddServers: 添加server信息
func (g *goParsePB) AddServers(servers ...*Server) {
	g.servers = append(g.servers, servers...)
}

// AddMessage: 添加message信息
func (g *goParsePB) AddMessages(messages ...*Message) {
	g.messages = append(g.messages, messages...)
}

// PackageName: 返回包名
func (g *goParsePB) PackageName() string {
	return g.pkgName
}

// GeneratePB: 生成pb文件
func (g *goParsePB) GeneratePB() string {
	if err := g.checkFormat(); err != nil {
		fmt.Println(err)
		return ""
	}
	var temp = `syntax = "proto3";
package {{.PackageName}};

// message{{range .messages}}
message {{.Name}}{
{{range  $index, $messages :=.Files}}   {{$messages.TypePB}} {{$messages.Name}} = {{addOne $index}};
{{end}}}
{{end}}

// server
service {{.meta.ServerName}}{
{{range .servers}}  rpc {{.Name }} ({{.InputParameter}}) returns ({{.OutputParameter}}) {
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
