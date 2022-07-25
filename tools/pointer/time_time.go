package pointer

import "time"

// NowTimePointer 返回当前时间的指针表示形式 *time.Time
func NowTimePointer() *time.Time {
	f := time.Now()
	return &f
}

// ToTimePointer 将time.Time类型转换为指针，如果时间为零值则返回空指针
func ToTimePointer(t time.Time) *time.Time {
	if t.IsZero() {
		return nil
	}
	return &t
}

// FromTimePointer 从time.Time类型的指针中读取时间，如果为空指针，则读取到零值
func FromTimePointer(t *time.Time) time.Time {
	return FromTimePointerOrDefault(t, time.Time{})
}

// FromTimePointerOrDefault 从time.Time类型的指针中读取时间，如果为空指针，则返回defaultValue
func FromTimePointerOrDefault(t *time.Time, defaultValue time.Time) time.Time {
	if t == nil {
		return defaultValue
	}
	return *t
}
