package goid

import (
	"fmt"
	"runtime"
	"strconv"
	"strings"
)

// GetGID 获取goroutine唯一ID
func GetGID() int64 {
	var buf [64]byte
	n := runtime.Stack(buf[:], false)
	// 得到id字符串
	idField := strings.Fields(strings.TrimPrefix(string(buf[:n]), "goroutine "))[0]
	id, err := strconv.Atoi(idField)
	if err != nil {
		panic(fmt.Sprintf("cannot get goroutine id: %v", err))
	}
	return int64(id)
}
