package parser

// package parse: 根据 go struct 转化成pb文件并构建注册代码

type Parser interface {
	PackageName() string
	Generate() string
}
