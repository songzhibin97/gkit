package generator

import (
	"time"
)

func ExampleNewSnowflake() {
	// 生成对象
	generate := NewSnowflake(time.Now(), 1)
	nid, err := generate.NextID()
	if err != nil {
		// 处理错误
	}
	_ = nid
}
