package group

// LazyLoadGroup 懒加载结构化
type LazyLoadGroup interface {
	Get(key string) interface{}
	ReSet(nf func() interface{})
	Clear()
}
