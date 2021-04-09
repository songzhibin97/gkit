package parsePb

import (
	"github.com/songzhibin97/gkit/cache/buffer"
	"fmt"
	"github.com/emicklei/proto"
	"text/template"
)

type PbParseGo struct {
	PkgName  string            // PkgName: 包名
	FilePath string            // FilePath: 文件的路径
	Server   []*Server         // Server: 解析出来function的信息
	Message  []*Message        // Message: 解析出struct的信息
	Note     []*Note           // Note: 其他注释
	Metas    map[string]string // Metas: 其他元信息
}

// CreatePbParseGo: 创建 PbParseGo
func CreatePbParseGo() *PbParseGo {
	return &PbParseGo{}
}

type Note struct {
	IsUse bool // 判断作用域, 如果是 struct中 或者 func中代表已经使用
	*proto.Comment
}

type Server struct {
	Offset          int              // Offset: 函数起始位置
	Name            string           // Name: 函数名
	ServerName      string           // ServerName: server name 通过 parseFunc 绑定
	Method          string           // Method: method 通过 parseFunc 绑定
	Router          string           // Router: router 通过 parseFunc 绑定
	InputParameter  string           // InputParameter: 入参
	OutputParameter string           // OutputParameter: 出参
	Doc             []string         // Doc: 函数注释信息,可以通过自定义的 parseFunc 去进行解析
	Notes           []*proto.Comment // Notes: 函数中的注释信息,用于埋点打桩
}

// CreateServer: 创建Server
func CreateServer(name string, offset int, inputParameter string, outputParameter string) *Server {
	return &Server{
		Offset:          offset,
		Name:            name,
		InputParameter:  inputParameter,
		OutputParameter: outputParameter,
	}
}

// Message: Message对应struct
type Message struct {
	Offset int              // Offset: message起始点
	Name   string           // Name: struct name
	Files  []*File          // Files: 字段信息
	Notes  []*proto.Comment // Notes: struct的注释信息,用于埋点打桩
}

func CreateMessage(name string, offset int) *Message {
	return &Message{
		Name:   name,
		Offset: offset,
	}
}

func (m *Message) AddFiles(files ...*File) {
	m.Files = append(m.Files, files...)
}

// File: 字段信息
type File struct {
	Name   string // Name: 字段名
	TypeGo string // TypeGo: 字段的原始类型
	TypePB string // TypePB: 字段在proto中的类型
}

// CreateFile: 创建字段信息
func CreateFile(name string, tGo string, tPb string) *File {
	return &File{
		Name:   name,
		TypeGo: tGo,
		TypePB: tPb,
	}
}

// AddServers: 添加server信息
func (p *PbParseGo) AddServers(servers ...*Server) {
	p.Server = append(p.Server, servers...)
}

// AddMessage: 添加message信息
func (p *PbParseGo) AddMessages(messages ...*Message) {
	p.Message = append(p.Message, messages...)
}

func (p *PbParseGo) parseMessage(ms *proto.Message, parseNote ...func(message *Message)) {
	ret := CreateMessage(ms.Name, ms.Position.Offset)
	// note
	if ms.Comment != nil {
		ret.Notes = append(ret.Notes, ms.Comment)
	}
	for _, element := range ms.Elements {
		switch v := element.(type) {
		case *proto.NormalField:
			// 正常的字段
			if v.Repeated {
				ret.AddFiles(CreateFile(v.Name, fmt.Sprintf("[]%s", PbTypeToGo(v.Type)), fmt.Sprintf("require %s", v.Type)))
			} else {
				ret.AddFiles(CreateFile(v.Name, PbTypeToGo(v.Type), v.Type))
			}

		case *proto.MapField:
			keyType := v.KeyType
			valueType := v.Field.Type
			ret.AddFiles(CreateFile(v.Field.Name, fmt.Sprintf("map[%s]%s",
				PbTypeToGo(keyType), PbTypeToGo(valueType)), fmt.Sprintf("<%s,%s>", keyType, valueType)))
		}
	}
	for _, f := range parseNote {
		f(ret)
	}
	p.AddMessages(ret)
}

func (p *PbParseGo) parseService(sv *proto.Service, parseDoc ...func(server *Server)) {
	for _, element := range sv.Elements {
		switch v := element.(type) {
		case *proto.RPC:
			funcName := v.Name
			reqType := v.RequestType
			retType := v.ReturnsType
			server := CreateServer(funcName, v.Position.Offset, PbTypeToGo(reqType), PbTypeToGo(retType))
			if sv.Comment != nil {
				server.Notes = append(server.Notes, sv.Comment)
				for _, doc := range sv.Comment.Lines {
					server.Doc = append(server.Doc, doc)
				}
				for _, f := range parseDoc {
					f(server)
				}
			}
			p.AddServers(server)
		}
	}
}

func (p *PbParseGo) PackageName() string {
	return p.PkgName
}

func (p *PbParseGo) Generate() string {
	var temp = `package {{.PkgName}}

// struct{{range .Message}}
type {{.Name}} struct {
{{range  $index, $Message :=.Files}}   {{$Message.Name}} {{$Message.TypeGo}}
{{end}}}
{{end}}

// function{{range .Server}}  
func {{.Name }} ({{.InputParameter}}) {{.OutputParameter}} {
   panic("Realize Me")
}
{{end}}
`
	tmpl, err := template.New("GeneratePB").Funcs(template.FuncMap{"addOne": addOne}).Parse(temp)
	if err != nil {
		return ""
	}
	b := buffer.NewIoBuffer(1024)
	err = tmpl.Execute(b, p)
	if err != nil {
		return ""
	}
	return b.String()
}
