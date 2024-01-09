package mutex

import (
	"fmt"
	"sync"
	"sync/atomic"
)

// TokenRecursiveMutex Token方式的递归锁
type TokenRecursiveMutex struct {
	sync.Mutex
	token     int64
	recursion int64
}

// Lock 请求锁，需要传入token
func (m *TokenRecursiveMutex) Lock(token int64) {
	if atomic.LoadInt64(&m.token) == token { // 如果传入的token和持有锁的token一致，说明是递归调用
		atomic.AddInt64(&m.recursion, 1)
		return
	}
	m.Mutex.Lock() // 传入的token不一致，说明不是递归调用
	// 抢到锁之后记录这个token
	atomic.StoreInt64(&m.token, token)
	atomic.StoreInt64(&m.recursion, 1)
}

// Unlock 释放锁
func (m *TokenRecursiveMutex) Unlock(token int64) {
	if atomic.LoadInt64(&m.token) != token { // 释放其它token持有的锁
		panic(fmt.Sprintf("wrong the owner(%d): %d!", m.token, token))
	}
	recursion := atomic.AddInt64(&m.recursion, -1)
	if recursion != 0 { // 如果这个goroutine还没有完全释放，则直接返回
		return
	}
	atomic.StoreInt64(&m.token, 0) // 没有递归调用了，释放锁
	m.Mutex.Unlock()
}
