package safe

import "unsafe"

// SliceModel slice底层模型
type SliceModel struct {
	Data unsafe.Pointer
	Len  int
	Cap  int
}

// StringModel string底层模型
type StringModel struct {
	Data unsafe.Pointer
	Len  int
}
