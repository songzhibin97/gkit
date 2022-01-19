package task

import "sync"

// Meta task可以携带元信息
type Meta struct {
	meta map[string]interface{}
	sync.RWMutex
	safe bool
}

// NewMeta 生成meta信息
func NewMeta(safe bool) *Meta {
	return &Meta{
		meta: make(map[string]interface{}),
		safe: safe,
	}
}

// Set 如果存在会覆盖
func (m *Meta) Set(key string, value interface{}) {
	if m.safe {
		m.Lock()
		defer m.Unlock()
	}
	m.meta[key] = value
}

func (m *Meta) Get(key string) (interface{}, bool) {
	if m.safe {
		m.RLock()
		defer m.RUnlock()
	}
	v, ok := m.meta[key]
	return v, ok
}

func (m *Meta) Range(f func(key string, value interface{})) {
	if m.safe {
		m.Lock()
		defer m.Unlock()
	}
	for k, v := range m.meta {
		f(k, v)
	}
}
