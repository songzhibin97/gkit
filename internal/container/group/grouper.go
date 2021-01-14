package group

// Grouper: 懒加接口化
type Grouper interface {
	Get(key string) interface{}
	ReSet(nf func() interface{})
}
