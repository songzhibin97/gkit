package task

import (
	"encoding/json"
	"sync"
)

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

func (m *Meta) normalize(safe bool) {
	if m.meta == nil {
		m.meta = make(map[string]interface{})
	}
	m.safe = safe
}

func (m *Meta) snapshot() map[string]interface{} {
	if m == nil {
		return nil
	}
	if m.safe {
		m.RLock()
		defer m.RUnlock()
	}
	values := make(map[string]interface{}, len(m.meta))
	for key, value := range m.meta {
		values[key] = value
	}
	return values
}

func (m *Meta) clone(safe bool) *Meta {
	values := m.snapshot()
	if values == nil {
		values = make(map[string]interface{})
	}
	return &Meta{meta: values, safe: safe}
}

// MarshalJSON serializes only metadata values. Synchronization state is local
// process state and must never be copied onto the wire.
func (m *Meta) MarshalJSON() ([]byte, error) {
	return json.Marshal(m.snapshot())
}

// UnmarshalJSON restores metadata values into a fresh map. Signature
// unmarshalling applies the owning signature's MetaSafe setting afterwards.
func (m *Meta) UnmarshalJSON(data []byte) error {
	var values map[string]interface{}
	if err := json.Unmarshal(data, &values); err != nil {
		return err
	}
	if values == nil {
		values = make(map[string]interface{})
	}
	if m.safe {
		m.Lock()
		defer m.Unlock()
	}
	m.meta = values
	return nil
}

// Set 如果存在会覆盖
func (m *Meta) Set(key string, value interface{}) {
	if m == nil {
		return
	}
	if m.safe {
		m.Lock()
		defer m.Unlock()
	}
	if m.meta == nil {
		m.meta = make(map[string]interface{})
	}
	m.meta[key] = value
}

func (m *Meta) Get(key string) (interface{}, bool) {
	if m == nil {
		return nil, false
	}
	if m.safe {
		m.RLock()
		defer m.RUnlock()
	}
	v, ok := m.meta[key]
	return v, ok
}

func (m *Meta) Range(f func(key string, value interface{})) {
	if m == nil {
		return
	}
	if m.safe {
		m.RLock()
		defer m.RUnlock()
	}
	for k, v := range m.meta {
		f(k, v)
	}
}
