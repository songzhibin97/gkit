package singleflight

// getResources 一般用于去数据库去获取数据
func getResources() (interface{}, error) {
	return "test", nil
}

// cache 填充到 缓存中的数据
func cache(v interface{}) {
	return
}

// ExampleNewSingleFlight
func ExampleNewSingleFlight() {
	singleFlight := NewSingleFlight()

	// 如果在key相同的情况下, 同一时间只有一个 func 可以去执行,其他的等待
	// 多用于缓存失效后,构造缓存,缓解服务器压力

	// 同步:
	v, err, _ := singleFlight.Do("test1", func() (interface{}, error) {
		// todo 这里去获取资源
		return getResources()
	})
	if err != nil {
		// todo 处理错误
	}
	// v 就是获取到的资源
	cache(v)

	// 异步:
	ch := singleFlight.DoChan("test2", func() (interface{}, error) {
		// todo 这里去获取资源
		return getResources()
	})

	result := <-ch
	if result.Err != nil {
		// todo 处理错误
	}
	cache(result.Val)

	// 尽力取消
	singleFlight.Forget("test2")
}
