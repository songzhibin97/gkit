package mutex

import (
	"fmt"
	"sync"
)

// TokenRecursiveMutex Token方式的递归锁
type TokenRecursiveMutex struct {
	sync.Mutex
	state     sync.Mutex
	token     int64
	recursion int64
	held      bool
}

// Lock 请求锁，需要传入token
func (m *TokenRecursiveMutex) Lock(token int64) {
	m.state.Lock()
	if m.held && m.token == token {
		m.recursion++
		m.state.Unlock()
		return
	}
	m.state.Unlock()

	m.Mutex.Lock()
	m.state.Lock()
	m.token = token
	m.recursion = 1
	m.held = true
	m.state.Unlock()
}

// Unlock 释放锁
func (m *TokenRecursiveMutex) Unlock(token int64) {
	m.state.Lock()
	if !m.held || m.token != token {
		owner := m.token
		m.state.Unlock()
		panic(fmt.Sprintf("wrong the owner(%d): %d!", owner, token))
	}
	m.recursion--
	if m.recursion != 0 {
		m.state.Unlock()
		return
	}
	m.held = false
	m.state.Unlock()
	m.Mutex.Unlock()
}
