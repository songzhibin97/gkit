package group

func createResources() interface{} {
	return map[int]int{}
}

func createResources2() interface{} {
	return []int{}
}

var group LazyLoadGroup

func ExampleNewGroup() {
	// 类似 sync.Pool 一样
	// 初始化一个group
	group = NewGroup(createResources)
}

func ExampleGroup_Get() {
	// 如果key 不存在 调用 NewGroup 传入的 function 创建资源
	// 如果存在则返回创建的资源信息
	v := group.Get("test")
	_ = v
}

func ExampleGroup_ReSet() {
	// ReSet 重置初始化函数,同时会对缓存的 key进行清空
	group.ReSet(createResources2)
}

func ExampleGroup_Clear() {
	// 清空缓存的 buffer
	group.Clear()
}
