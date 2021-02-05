package generator

import (
	"time"
)

func ExampleNewSnowflake() {
	// 生成对象
	ids := NewSnowflake(time.Now(), 1)
	nid, err := ids.NextID()
	if err != nil {
		// 处理错误
	}
	_ = nid
}
