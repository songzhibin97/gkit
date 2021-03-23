package parse

// package parse: 根据 go struct 转化成pb文件并构建注册代码

type Parse interface {
	PackageName() string
	AddServers(...*Server)
	Servers() []*Server
	AddMessages(...*Message)
	Messages() []*Message
	Notes() []*Note
	AddNotes(...*Note)
	Generate() string
}
