package locker

import "context"

// 分布式锁

type Locker interface {
	// Lock 获取锁
	// key 锁名称
	// mark 锁的凭证,用于释放锁的唯一标志
	// expire 锁过期失效,以Millisecond为单位 1000 = 1s
	Lock(key string, expire int, mark string) error

	// UnLock 解锁
	// key 锁名称
	// mark 锁的凭证,用于释放锁的唯一标志
	UnLock(key string, mark string) error

	// Renew 续约锁,延长锁的过期时间
	// key 锁名称
	// mark 锁的凭证,用于验证是否为锁的持有者
	// expire 新的过期时间,以Millisecond为单位 1000 = 1s
	// 返回 error 如果续约失败(锁不存在或mark不匹配)
	Renew(key string, expire int, mark string) error
}

// ContextLocker is an optional capability for operations that must be joined
// during server shutdown. Locker remains unchanged for third-party source
// compatibility.
type ContextLocker interface {
	LockContext(ctx context.Context, key string, expire int, mark string) error
	UnlockContext(ctx context.Context, key string, mark string) error
}
