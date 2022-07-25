package pointer

import "time"

// ToDurationPointer 将time.Duration类型的变量转换为对应的*time.Duration指针类型
func ToDurationPointer(v time.Duration) *time.Duration {
	return &v
}

// ToDurationPointerOrNilIfZero 将time.Duration类型的变量转换为对应的*time.Duration指针类型，如果变量的值为0的话则返回nil指针
func ToDurationPointerOrNilIfZero(v time.Duration) *time.Duration {
	if v == 0 {
		return nil
	}
	return &v
}

// FromDurationPointer 获取*time.Duration类型的指针的实际值，如果指针为nil的话则返回0
func FromDurationPointer(p *time.Duration) time.Duration {
	return FromDurationPointerOrDefaultIfNil(p, 0)
}

// FromDurationPointerOrDefaultIfNil 获取*time.Duration类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromDurationPointerOrDefaultIfNil(v *time.Duration, defaultValue time.Duration) time.Duration {
	if v == nil {
		return defaultValue
	}
	return *v
}
