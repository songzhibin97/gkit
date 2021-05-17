package window

// SlidingWindow 滑动窗口接口化
type SlidingWindow interface {
	// sentinel 哨兵
	sentinel()

	// Shutdown 优雅关闭
	Shutdown()

	// AddIndex 添加指标信息
	AddIndex(k string, v uint)

	// Show 展示信息
	Show() []interface{}
}
