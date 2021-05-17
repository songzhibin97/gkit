package group

import "sync"

// package group: 提供懒加载容器

// Group 懒加载容器
type Group struct {
	sync.RWMutex
	f    func() interface{}
	objs map[string]interface{}
}

// Get 根据key 获取 value
func (g *Group) Get(key string) interface{} {
	g.RLock()
	if obj, ok := g.objs[key]; ok {
		g.RUnlock()
		return obj
	}
	g.RUnlock()

	g.Lock()
	defer g.Unlock()
	// 再次判断
	if obj, ok := g.objs[key]; ok {
		return obj
	}

	obj := g.f()
	g.objs[key] = obj
	return obj
}

// ReSet 更换实例化函数
func (g *Group) ReSet(nf func() interface{}) {
	if nf == nil {
		panic("container.group: 不能为新函数分配nil")
	}
	g.Lock()
	g.f = nf
	g.Unlock()
	g.Clear()
}

func (g *Group) Clear() {
	g.Lock()
	defer g.Unlock()
	g.objs = make(map[string]interface{})
}

// NewGroup Group 实例化方法
func NewGroup(f func() interface{}) LazyLoadGroup {
	if f == nil {
		panic("container.group: 不能为新函数分配nil")
	}
	return &Group{
		f:    f,
		objs: make(map[string]interface{}),
	}
}
