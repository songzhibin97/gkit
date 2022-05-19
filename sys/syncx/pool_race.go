//go:build race
// +build race

package syncx

import (
	"sync"
)

type Pool struct {
	p    sync.Pool
	once sync.Once
	// New optionally specifies a function to generate
	// a value when Get would otherwise return nil.
	// It may not be changed concurrently with calls to Get.
	New func() interface{}
	// NoGC any objects in this Pool.
	NoGC bool
}

func (p *Pool) init() {
	p.once.Do(func() {
		p.p = sync.Pool{
			New: p.New,
		}
	})
}

// Put adds x to the pool.
func (p *Pool) Put(x interface{}) {
	p.init()
	p.p.Put(x)
}

// Get selects an arbitrary item from the Pool, removes it from the
// Pool, and returns it to the caller.
// Get may choose to ignore the pool and treat it as empty.
// Callers should not assume any relation between values passed to Put and
// the values returned by Get.
//
// If Get would otherwise return nil and p.New is non-nil, Get returns
// the result of calling p.New.
func (p *Pool) Get() (x interface{}) {
	p.init()
	return p.p.Get()
}
