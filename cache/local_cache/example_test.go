package local_cache

import (
	"log"

	"github.com/songzhibin97/gkit/cache/buffer"
)

var ch Cache

func ExampleNewCache() {
	// 默认配置
	// ch = NewCache()

	// 可供选择的配置选项

	// 设置间隔时间
	// SetInternal(interval time.Duration)

	// 设置默认的超时时间
	// SetDefaultExpire(expire time.Duration)

	// 设置周期的执行函数,默认(不设置)是扫描全局清除过期的k
	// SetFn(fn func())

	// 设置触发删除后的捕获函数, 数据删除后回调用设置的捕获函数
	// SetCapture(capture func(k string, v interface{}))

	// 设置初始化存储的成员对象
	// SetMember(m map[string]Iterator)

	ch = NewCache(SetInternal(1000),
		SetDefaultExpire(10000),
		SetCapture(func(k string, v interface{}) {
			log.Println(k, v)
		}))
}

func ExampleCacheStorage() {
	// Set 添加cache 无论是否存在都会覆盖
	ch.Set("k1", "v1", DefaultExpire)

	// SetDefault 无论是否存在都会覆盖
	// 偏函数模式,默认传入超时时间为创建cache的默认时间
	ch.SetDefault("k1", 1)

	// SetNoExpire
	// 偏函数模式,默认传入超时时间为永不过期
	ch.SetNoExpire("k1", 1.1)

	// Add 添加cache 如果存在的话会抛出异常
	err := ch.Add("k1", nil, DefaultExpire)
	CacheErrExist(err) // true

	// Replace 如果有就设置没有就抛出错误
	err = ch.Replace("k2", make(chan struct{}), DefaultExpire)
	CacheErrNoExist(err) // true
}

func ExampleGet() {
	// Get 根据key获取 cache 保证有效期内的kv被取出
	v, ok := ch.Get("k1")
	if !ok {
		// v == nil
	}
	_ = v

	// GetWithExpire 根据key获取 cache 并带出超时时间
	v, t, ok := ch.GetWithExpire("k1")
	if !ok {
		// v == nil
	}
	// 如果超时时间是 NoExpire t.IsZero() == true
	if t.IsZero() {
		// 没有设置超时时间
	}

	// Iterator 返回 cache 中所有有效的对象
	mp := ch.Iterator()
	for s, iterator := range mp {
		log.Println(s, iterator)
	}

	// Count 返回member数量
	log.Println(ch.Count())
}

func ExampleIncrement() {
	ch.Set("k3", 1, DefaultExpire)
	ch.Set("k4", 1.1, DefaultExpire)
	// Increment 为k对应的value增加n n必须为数字类型
	err := ch.Increment("k3", 1)
	if CacheErrExpire(err) || CacheErrExist(CacheTypeErr) {
		// 未设置成功
	}
	_ = ch.IncrementFloat("k4", 1.1)

	// 如果你知道设置的k的具体类型 还可以使用类型确定的 increment函数
	// ch.IncrementInt(k string, v int)
	// ...
	// ch.IncrementFloat32(k string, v flot32)
	// ...

	// Decrement 同理
}

func ExampleDelete() {
	// Delete 如果设置了 capture 会触发不或函数
	ch.Delete("k1")

	// DeleteExpire 删除所有过期了的key, 默认的 capture 就是执行 DeleteExpire()
	ch.DeleteExpire()
}

func ExampleChangeCapture() {
	// 提供了在运行中改变捕获函数的方法
	// ChangeCapture
	ch.ChangeCapture(func(k string, v interface{}) {
		log.Println(k, v)
	})
}

func ExampleSaveLoad() {
	// 写入文件采用go独有的gob协议

	io := buffer.NewIoBuffer(1000)

	// Save 传入一个 w io.Writer 参数 将 cache中的 member 成员写入w中
	_ = ch.Save(io)

	// SaveFile 传入path 写到文件中
	_ = ch.SaveFile("path")

	// Load 传入一个 r io.Reader对象 从 r中读取写回到 member中
	_ = ch.Load(io)

	// LoadFile 传入path 读取文件内容
	_ = ch.LoadFile("path")
}

func ExampleFlush() {
	// Flush 释放member成员
	ch.Flush()
}

func ExampleShutdown() {
	// Shutdown 释放对象
	_ = ch.Shutdown()
}
