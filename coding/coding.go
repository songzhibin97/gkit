package coding

import (
	"errors"
	"strings"
	"sync"
)

// package encoding 各种格式编码解码

var ErrorTypeCode = errors.New("coding: code type error")

var registerCode = CodeStorage{storage: map[string]Code{}}

type (
	// Code coding 接口
	Code interface {
		// Marshal 将v序列化为[]byte
		Marshal(v interface{}) ([]byte, error)

		// Unmarshal 将[]byte 反序列化为v
		Unmarshal(data []byte, v interface{}) error

		// Name 返回实际调用编码器的类型, 例如 json、xml、yaml、proto
		Name() string
	}

	// CodeStorage 注册中心
	CodeStorage struct {
		storage map[string]Code
		sync.Mutex
	}
)

func RegisterCode(code Code) error {
	if code == nil || len(code.Name()) == 0 {
		return ErrorTypeCode
	}
	registerCode.Lock()
	defer registerCode.Unlock()
	registerCode.storage[strings.ToLower(code.Name())] = code
	return nil
}

func GetCode(codeName string) Code {
	registerCode.Lock()
	defer registerCode.Unlock()
	return registerCode.storage[strings.ToLower(codeName)]
}
