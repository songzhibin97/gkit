package fileparse

type Parse interface {
	PackageName() string
	AddServers(...*Server)
	Servers() []*Server
	AddMessages(...*Message)
	Messages() []*Message
	GeneratePB() string
}
