package generator

// package generator: 发号器

// Generator 发号器
type Generator interface {
	// NextID 获取下一个ID
	NextID() (uint64, error)
}
