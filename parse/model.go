package parse

import "go/ast"

type Note struct {
	IsUse bool // 判断作用域, 如果是 struct中 或者 func中代表已经使用
	*ast.Comment
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
