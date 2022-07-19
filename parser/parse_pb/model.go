package parse_pb

import (
	"fmt"
	"strconv"
	"strings"
	"text/template"

	"github.com/emicklei/proto"
	"github.com/songzhibin97/gkit/cache/buffer"
	"github.com/songzhibin97/gkit/options"
)

type (
	ParseMessage func(m *Message)
	ParseService func(server *Server)
	// CheckFunc    func(p *PbParseGo) error
)

type PbParseGo struct {
	PkgName  string              // PkgName: 包名
	FilePath string              // FilePath: 文件的路径
	Server   map[string]*Server  // Server: 服务器信息
	Message  map[string]*Message // Message: 消息信息
	Note     map[string]*Note    // Note: 注释信息
	Enums    map[string]*Enum    // Enums: 枚举类型
	//Server        []*Server         // Server: 解析出来function的信息
	//Message       []*Message        // Message: 解析出struct的信息
	//Note          []*Note           // Note: 其他注释
	//Enums         []*Enum           // Enum: 解析出enum的信息
	Metas         map[string]string // Metas: 其他元信息
	ParseMessages []ParseMessage
	ParseServices []ParseService
}

// CreatePbParseGo 创建 PbParseGo
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

// CreateServer 创建Server
func CreateServer(name string, offset int, inputParameter string, outputParameter string) *Server {
	return &Server{
		Offset:          offset,
		Name:            name,
		InputParameter:  inputParameter,
		OutputParameter: outputParameter,
	}
}

// Message Message对应struct
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

// File 字段信息
type File struct {
	Name   string // Name: 字段名
	TypeGo string // TypeGo: 字段的原始类型
	TypePB string // TypePB: 字段在proto中的类型
}

// CreateFile 创建字段信息
func CreateFile(name string, tGo string, tPb string) *File {
	if name != "" {
		name = strings.ToTitle(string(name[0])) + name[1:]
	}

	return &File{
		Name:   name,
		TypeGo: tGo,
		TypePB: tPb,
	}
}

// addParseStruct 添加自定义解析struct内容
func (p *PbParseGo) addParseMessage(parseMessage ...ParseMessage) {
	p.ParseMessages = append(p.ParseMessages, parseMessage...)
}

// addParseFunc 添加自定义解析Func
func (p *PbParseGo) addParseService(parseService ...ParseService) {
	p.ParseServices = append(p.ParseServices, parseService...)
}

// AddServers 添加server信息
func (p *PbParseGo) AddServers(servers ...*Server) {
	if p.Server == nil {
		p.Server = make(map[string]*Server)
	}
	for _, server := range servers {
		p.Server[server.Name] = server
	}
}

// AddMessages 添加message信息
func (p *PbParseGo) AddMessages(messages ...*Message) {
	if p.Message == nil {
		p.Message = make(map[string]*Message)
	}
	for _, message := range messages {
		p.Message[message.Name] = message
	}
}

func (p *PbParseGo) AddNode(nodes ...*Note) {
	if p.Note == nil {
		p.Note = make(map[string]*Note)
	}
	for _, node := range nodes {
		p.Note[strconv.Itoa(node.Position.Offset)] = node
	}
}

// AddEnum 添加枚举类型
func (p *PbParseGo) AddEnum(enums ...*Enum) {
	if p.Enums == nil {
		p.Enums = make(map[string]*Enum)
	}
	for _, e := range enums {
		p.Enums[e.Name] = e
	}
}

type Enum struct {
	Offset   int    // Offset: 函数起始位置
	Name     string // Name: 类型名称
	Elements map[string]*EnumElement
}

// CreateEnum 创建枚举类型
func CreateEnum(name string, offset int) *Enum {
	return &Enum{
		Name:   name,
		Offset: offset,
	}
}

type EnumElement struct {
	Offset int    // Offset: 函数起始位置
	Name   string // Name: 类型名称
	Index  int    // Index: 索引
}

func (e *Enum) AddElem(name string, offset int, index int) {
	if e.Elements == nil {
		e.Elements = make(map[string]*EnumElement)
	}
	e.Elements[name] = &EnumElement{
		Name:   name,
		Offset: offset,
		Index:  index,
	}
}

func (p *PbParseGo) parseMessage(ms *proto.Message, prefix string) {
	ret := CreateMessage(prefix+ms.Name, ms.Position.Offset)
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
		case *proto.Message:
			p.parseMessage(v, ms.Name+prefix)
		case *proto.Enum:
			ret.AddFiles(CreateFile(v.Name, v.Name, "enum"))
			p.parseEnum(v, ms.Name+prefix)
		}
	}
	for _, f := range p.ParseMessages {
		f(ret)
	}
	p.AddMessages(ret)
}

func (p *PbParseGo) parseService(sv *proto.Service) {
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
				for _, f := range p.ParseServices {
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
	temp := `package {{.PkgName}}
// type{{range .Enums}}	
type {{.Name}} int32
{{ $Type := .Name}}const({{range $index, $Elem := .Elements}}
{{$Elem.Name}} {{ $Type }} = {{$Elem.Index}}{{end}}
)
{{end}}	
	
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

func (p *PbParseGo) parseEnum(sv *proto.Enum, prefix string) {

	enum := CreateEnum(prefix+sv.Name, sv.Position.Offset)
	for _, element := range sv.Elements {
		switch v := element.(type) {
		case *proto.EnumField:
			enum.AddElem(v.Name, v.Position.Offset, v.Integer)
		}
	}
	p.AddEnum(enum)
}

// AddParseMessage 添加自定义解析message
func AddParseMessage(parseMessages ...ParseMessage) options.Option {
	return func(o interface{}) {
		o.(*PbParseGo).addParseMessage(parseMessages...)
	}
}

// AddParseService 添加自定义解析service
func AddParseService(parseServices ...ParseService) options.Option {
	return func(o interface{}) {
		o.(*PbParseGo).addParseService(parseServices...)
	}
}
