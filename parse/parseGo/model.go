package parseGo

import (
	"Songzhibin/GKit/cache/buffer"
	"Songzhibin/GKit/internal/sys/mutex"
	"Songzhibin/GKit/parse"
	"errors"
	"fmt"
	"go/ast"
	"io/ioutil"
	"strings"
	"text/template"
)

// GoParsePB: .go 文件转成 pb文件
type GoParsePB struct {
	mutex.Mutex
	PkgName  string            // PkgName: 包名
	FilePath string            // FilePath: 文件的路径
	Server   []*parse.Server   // Server: 解析出来function的信息
	Message  []*parse.Message  // Message: 解析出struct的信息
	Note     []*parse.Note     // Note: 其他注释
	Metas    map[string]string // Metas: 其他元信息
}

// CreateGoParsePB: 创建 goParsePB Metas
func CreateGoParsePB(pkgName string, filepath string, notes []*parse.Note) parse.Parse {
	return &GoParsePB{
		PkgName:  pkgName,
		FilePath: filepath,
		Metas:    make(map[string]string),
		Note:     notes,
	}
}

// parseStruct: 解析struct信息
func (g *GoParsePB) parseStruct(st *ast.GenDecl, parseTag ...func(file *parse.File)) {
	for _, spec := range st.Specs {
		if v, ok := spec.(*ast.TypeSpec); ok {
			ret := parse.CreateMessage(v.Name.Name, int(v.Pos()), int(v.End()))
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
								ret.AddFiles(parse.CreateFile(tag, tPb, tGo, tPb))
							} else {
								ret.AddFiles(parse.CreateFile(tag, name, tGo, tPb))
							}

						case *ast.ArrayType:
							if aType, ok := tType.Elt.(*ast.Ident); ok {
								tGo := fmt.Sprintf(`[]%s`, aType.Name)
								if aType.Name == "byte" {
									tPb := `bytes`
									ret.AddFiles(parse.CreateFile(tag, name, tGo, tPb))
								} else {
									tPb := fmt.Sprintf(`repeated %s`, GoTypeToPB(aType.Name))
									ret.AddFiles(parse.CreateFile(tag, name, tGo, tPb))
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
								ret.AddFiles(parse.CreateFile(tag, name, tGo, tPb))
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
func (g *GoParsePB) parseFunc(fn *ast.FuncDecl, parseDocs ...func(*parse.Server)) {
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
	ret := parse.CreateServer(name, int(fn.Pos()), int(fn.End()), tags, inputParameter, outputParameter)
	for _, f := range parseDocs {
		f(ret)
	}
	g.AddServers(ret)
}

// checkFormat: 查重,以及确认服务中的出参入参是否在上文中出现
func (g *GoParsePB) checkFormat() error {
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
	serverHashSet := make(map[string]struct{})
	for _, serve := range g.Server {
		if serve.ServerName != "" {
			g.Metas["ServerName"] = serve.ServerName
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
		for _, note := range g.Note {
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
func (g *GoParsePB) Servers() []*parse.Server {
	return g.Server
}

// Messages: 返回解析后的所有Message对象
func (g *GoParsePB) Messages() []*parse.Message {
	return g.Message
}

// AddServers: 添加server信息
func (g *GoParsePB) AddServers(servers ...*parse.Server) {
	g.Server = append(g.Server, servers...)
}

// AddMessage: 添加message信息
func (g *GoParsePB) AddMessages(messages ...*parse.Message) {
	g.Message = append(g.Message, messages...)
}

// Notes: 获取注释消息
func (g *GoParsePB) Notes() []*parse.Note {
	return g.Note
}

func (g *GoParsePB) AddNotes(notes ...*parse.Note) {
	g.Note = append(g.Note, notes...)
}

// PackageName: 返回包名
func (g *GoParsePB) PackageName() string {
	return g.PkgName
}

// Generate: 生成pb文件
func (g *GoParsePB) Generate() string {
	var temp = `syntax = "proto3";
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

// PileDriving: 源文件打桩
// functionName: 指定函数内打桩,选传
// startNotes,endNotes: 可以传两个打桩点,startNotes,endNotes中必填一个
// insertCode: 插入代码段
func (g *GoParsePB) PileDriving(functionName string, startNotes, endNotes string, insertCode string) error {
	// srcData: 源文件内容
	srcData, err := ioutil.ReadFile(g.FilePath)
	if err != nil {
		return err
	}
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
		return errors.New("startNotes and endNotes is not find")
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
	var (
		sym     []byte
		oldTail []byte
	)
	if endNotesPos == len(srcData)+1 {
		endNotesPos = startNotesPos
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

	return ioutil.WriteFile(g.FilePath, srcData, 0600)
}
