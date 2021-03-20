package fileparse

type Parse interface {
	PackageName() string
	Servers() []*Server
	Messages() []*Message
	GeneratePB() string
}
