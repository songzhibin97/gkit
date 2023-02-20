package locker

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
}
