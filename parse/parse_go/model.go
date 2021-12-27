package parse_go

import (
	"errors"
	"fmt"
	"go/ast"
	"io/ioutil"
	"strings"
	"text/template"

	"github.com/songzhibin97/gkit/cache/buffer"
	"github.com/songzhibin97/gkit/options"
)

type (
	ParseStruct func(file *File)
	ParseFunc   func(server *Server)
	CheckFunc   func(g *GoParsePB) error
)

// GoParsePB .go 文件转成 pb文件
type GoParsePB struct {
	PkgName      string            // PkgName: 包名
	FilePath     string            // FilePath: 文件的路径
	Server       []*Server         // Server: 解析出来function的信息
	Message      []*Message        // Message: 解析出struct的信息
	Note         []*Note           // Note: 其他注释
	Metas        map[string]string // Metas: 其他元信息
	ParseStructs []ParseStruct
	ParseFuncS   []ParseFunc
	CheckFuncS   []CheckFunc
}

type Note struct {
	IsUse bool // 判断作用域, 如果是 struct中 或者 func中代表已经使用
	*ast.Comment
}

// Server Server对应Go func
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

// CreateServer 创建Server
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

// Message Message对应struct
type Message struct {
	Pos   int            // Pos: struct的起始字节位置
	End   int            // End: struct的结束字节为止
	Name  string         // Name: struct name
	Files []*File        // Files: 字段信息
	Notes []*ast.Comment // Notes: struct的注释信息,用于埋点打桩
}

// AddFiles 添加字段信息
func (m *Message) AddFiles(files ...*File) {
	m.Files = append(m.Files, files...)
}

// CreateMessage 创建Message
func CreateMessage(name string, pos, end int) *Message {
	return &Message{
		Name: name,
		Pos:  pos,
		End:  end,
	}
}

// File 字段信息
type File struct {
	Tag    string // Tag: 字段的tag标记
	Name   string // Name: 字段名
	TypeGo string // TypeGo: 字段的原始类型
	TypePB string // TypePB: 字段在proto中的类型
}

// CreateFile 创建字段信息
func CreateFile(tag string, name string, tGo string, tPb string) *File {
	return &File{
		Tag:    tag,
		Name:   name,
		TypeGo: tGo,
		TypePB: tPb,
	}
}

// CreateGoParsePB 创建 GoParsePB Metas
func CreateGoParsePB(pkgName string, filepath string, notes []*Note) *GoParsePB {
	return &GoParsePB{
		PkgName:  pkgName,
		FilePath: filepath,
		Metas:    make(map[string]string),
		Note:     notes,
	}
}

// addParseStruct 添加自定义解析struct内容
func (g *GoParsePB) addParseStruct(parseTag ...ParseStruct) {
	g.ParseStructs = append(g.ParseStructs, parseTag...)
}

// addParseFunc 添加自定义解析Func
func (g *GoParsePB) addParseFunc(parseDocs ...ParseFunc) {
	g.ParseFuncS = append(g.ParseFuncS, parseDocs...)
}

// addCheck 添加后续校验信息
func (g *GoParsePB) addCheck(checkFunc ...CheckFunc) {
	g.CheckFuncS = append(g.CheckFuncS, checkFunc...)
}

// parseStruct 解析struct信息
func (g *GoParsePB) parseStruct(st *ast.GenDecl) {
	for _, spec := range st.Specs {
		if v, ok := spec.(*ast.TypeSpec); ok {
			ret := CreateMessage(v.Name.Name, int(v.Pos()), int(v.End()))
			if sType, ok := v.Type.(*ast.StructType); ok {

				for _, field := range sType.Fields.List {
					var tag, name string
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
				for _, f := range g.ParseStructs {
					for _, file := range ret.Files {
						f(file)
					}
				}

				g.AddMessages(ret)
			}
		}
	}
}

// parseFunc 解析函数信息
func (g *GoParsePB) parseFunc(fn *ast.FuncDecl) {
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
	for _, f := range g.ParseFuncS {
		f(ret)
	}
	g.AddServers(ret)
}

// checkFormat 简单处理meta信息,将对应func、server中的注释移入
func (g *GoParsePB) checkFormat() error {
	// 之前已经调用过了,就直接返回了
	if _, ok := g.Metas["ServerName"]; ok {
		return nil
	}
	msgHashSet := make(map[string]struct{})

	for _, message := range g.Message {
		if _, ok := msgHashSet[message.Name]; ok {
			return errors.New("message repeat")
		}
		for _, note := range g.Note {
			if !note.IsUse && int(note.Pos()) > message.Pos && int(note.End()) <= message.End {
				message.Notes = append(message.Notes, note.Comment)
				note.IsUse = true
			}
		}
		msgHashSet[message.Name] = struct{}{}
	}
	// serverHashSet := make(map[string]struct{})
	for _, serve := range g.Server {
		if serve.ServerName != "" {
			g.Metas["ServerName"] = serve.ServerName
		}
		//if serve.Router == "" || serve.Method == "" {
		//	return errors.New("server router or method is empty")
		//}
		//if _, ok := serverHashSet[serve.Router+serve.Method]; ok {
		//	return errors.New("server router method repeat")
		//}
		//if _, ok := msgHashSet[serve.InputParameter]; !ok {
		//	return errors.New("server input Parameters is empty")
		//}
		//if _, ok := msgHashSet[serve.OutputParameter]; !ok {
		//	return errors.New("server output Parameters is empty")
		//}
		for _, note := range g.Note {
			if !note.IsUse && int(note.Pos()) > serve.Pos && int(note.End()) <= serve.End {
				serve.Notes = append(serve.Notes, note.Comment)
				note.IsUse = true
			}
		}
	}

	for _, checkFunc := range g.CheckFuncS {
		if err := checkFunc(g); err != nil {
			return err
		}
	}

	return nil
}

// Servers 返回解析后的所有Server对象
func (g *GoParsePB) Servers() []*Server {
	return g.Server
}

// Messages 返回解析后的所有Message对象
func (g *GoParsePB) Messages() []*Message {
	return g.Message
}

// AddServers 添加server信息
func (g *GoParsePB) AddServers(servers ...*Server) {
	g.Server = append(g.Server, servers...)
}

// AddMessages 添加message信息
func (g *GoParsePB) AddMessages(messages ...*Message) {
	g.Message = append(g.Message, messages...)
}

// Notes 获取注释消息
func (g *GoParsePB) Notes() []*Note {
	return g.Note
}

func (g *GoParsePB) AddNotes(notes ...*Note) {
	g.Note = append(g.Note, notes...)
}

// PackageName 返回包名
func (g *GoParsePB) PackageName() string {
	return g.PkgName
}

// Generate 生成pb文件
func (g *GoParsePB) Generate() string {
	temp := `syntax = "proto3";
package {{.PackageName}};

// message{{range .Message}}
message {{.Name}}{
{{range  $index, $Message :=.Files}}   {{$Message.TypePB}} {{$Message.Name}} = {{addOne $index}};
{{end}}}
{{end}}

// server
service {{.Metas.ServerName}}{
{{range .Server}}  rpc {{.Name }} ({{.InputParameter}}) returns ({{.OutputParameter}}) {
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

// PileDriving 源文件打桩
// functionName: 指定函数内打桩,选传
// startNotes,endNotes: 可以传两个打桩点,startNotes,endNotes中必填一个
// insertCode: 插入代码段
func (g *GoParsePB) PileDriving(functionName string, startNotes, endNotes string, insertCode string) error {
	// srcData: 源文件内容
	srcData, err := ioutil.ReadFile(g.FilePath)
	if err != nil {
		return err
	}
	startNotesPos, endNotesPos, err := g.pileFind(srcData, functionName, startNotes, endNotes)
	srcData, err = g.pileDriving(srcData, startNotesPos, endNotesPos, insertCode)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(g.FilePath, srcData, 0o600)
}

func (g *GoParsePB) PileDismantle(clearCode string) error {
	// srcData: 源文件内容
	srcData, err := ioutil.ReadFile(g.FilePath)
	if err != nil {
		return err
	}

	srcData, err = g.pileDismantle(srcData, clearCode)
	if err != nil {
		return err
	}
	return ioutil.WriteFile(g.FilePath, srcData, 0o600)
}

// pileFind: 找到打桩点,返回 startNotesPos、endNotesPos
func (g *GoParsePB) pileFind(srcData []byte, functionName string, startNotes, endNotes string) (int, int, error) {
	var (
		startNotesPos = -1
		endNotesPos   = len(srcData) + 1
	)
	// 判断是否指定functionName
	if len(functionName) > 0 {
		// 从函数中找桩
		for _, server := range g.Server {
			if server.Name != functionName {
				continue
			}
			// 遍历notes看是否匹配
			for _, note := range server.Notes {
				if startNotesPos != -1 && endNotesPos != len(srcData)+1 {
					break
				}
				if startNotesPos == -1 && strings.Contains(note.Text, startNotes) {
					startNotesPos = int(note.Pos())
				}
				if endNotesPos == len(srcData)+1 && strings.Contains(note.Text, endNotes) {
					endNotesPos = int(note.Pos())
				}
			}
		}
	} else {
		// 从全局注释里面找
		for _, note := range g.Note {
			if startNotesPos != -1 && endNotesPos != len(srcData)+1 {
				break
			}
			if startNotesPos == -1 && strings.Contains(note.Text, startNotes) {
				startNotesPos = int(note.Pos())
			}
			if endNotesPos == len(srcData)+1 && strings.Contains(note.Text, endNotes) {
				endNotesPos = int(note.Pos())
			}
		}
	}
	// 判断是否找到桩点
	if startNotesPos == -1 && endNotesPos == len(srcData)+1 {
		return 0, 0, errors.New("startNotes and endNotes is not find")
	}
	// 判断是否两个都找到
	if startNotesPos != -1 && endNotesPos != len(srcData)+1 {
		// 如果是同一行,需要处理
		if startNotesPos == endNotesPos {
			endNotesPos = startNotesPos + strings.Index(string(srcData[startNotesPos:]), startNotes)
			for srcData[endNotesPos] != '/' {
				endNotesPos--
			}
		}
	}

	return startNotesPos, endNotesPos, nil
}

// pileDriving: 打桩,返回已经打好的 srcData数据
func (g *GoParsePB) pileDriving(srcData []byte, startNotesPos, endNotesPos int, insertCode string) ([]byte, error) {
	var (
		sym     []byte
		oldTail []byte
	)
	if endNotesPos == len(srcData)+1 {
		endNotesPos = startNotesPos
		if checkRepeat(insertCode, string(srcData[endNotesPos:])) {
			return nil, errors.New("重复添加")
		}

		// 收集标记符
		endNotesPos--
		symStart := endNotesPos
		for symStart > 0 && srcData[symStart] != '\n' {
			symStart--
		}
		sym = make([]byte, endNotesPos-symStart)
		copy(sym, srcData[symStart:])

		for endNotesPos < len(srcData)-1 && srcData[endNotesPos] != '\n' {
			endNotesPos++
		}

		oldTail = make([]byte, len(srcData)-endNotesPos)
		copy(oldTail, srcData[endNotesPos:])

		srcData = srcData[:endNotesPos]
	} else {
		if checkRepeat(insertCode, string(srcData[:endNotesPos])) {
			return nil, errors.New("重复添加")
		}
		endNotesPos--
		symStart := endNotesPos
		for symStart > 0 && srcData[symStart] != '\n' {
			symStart--
		}
		sym = make([]byte, endNotesPos-symStart)
		copy(sym, srcData[symStart:])

		oldTail = make([]byte, len(srcData)-symStart)
		copy(oldTail, srcData[symStart:])

		srcData = srcData[:symStart]
	}
	srcData = append(srcData, sym...)
	srcData = append(srcData, []byte(insertCode)...)
	srcData = append(srcData, oldTail...)
	return srcData, nil
}

func (g *GoParsePB) pileDismantle(srcData []byte, clearCode string) ([]byte, error) {
	return cleanCode(clearCode, string(srcData))
}

// cleanCode: 清除桩内内容
func cleanCode(clearCode string, srcData string) ([]byte, error) {
	bf := make([]rune, 0, 1024)
	for i, v := range srcData {
		if v == '\n' {
			if strings.TrimSpace(string(bf)) == clearCode {
				return append([]byte(srcData[:i-len(bf)]), []byte(srcData[i+1:])...), nil
			}
			bf = (bf)[:0]
			continue
		}
		bf = append(bf, v)
	}
	return []byte(srcData), errors.New("未找到内容")
}

// checkRepeat 检查是否重复
func checkRepeat(code string, context string) bool {
	bf := make([]rune, 0, 1024)
	for _, v := range context {
		if v == '\n' {
			if strings.TrimSpace(string(bf)) == code {
				return true
			}
			bf = (bf)[:0]
			continue
		}
		bf = append(bf, v)
	}
	return false
}

// AddParseStruct 添加自定义解析struct内容
func AddParseStruct(parseTag ...ParseStruct) options.Option {
	return func(o interface{}) {
		o.(*GoParsePB).addParseStruct(parseTag...)
	}
}

// AddParseFunc 添加自定义解析Func
func AddParseFunc(parseDocs ...ParseFunc) options.Option {
	return func(o interface{}) {
		o.(*GoParsePB).addParseFunc(parseDocs...)
	}
}

// AddCheck 添加后续校验信息
func AddCheck(checkFuncs ...CheckFunc) options.Option {
	return func(o interface{}) {
		o.(*GoParsePB).addCheck(checkFuncs...)
	}
}
