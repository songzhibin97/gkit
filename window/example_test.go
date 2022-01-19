package window

func ExampleInitWindow() {
	// 初始化窗口
	w := NewWindow()

	// 增加指标
	// key:权重
	w.AddIndex("key", 1)

	// Show: 返回当前指标
	slice := w.Show()
	_ = slice
}
