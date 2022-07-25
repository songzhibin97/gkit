package pointer

// ToStringPointer 将string类型的变量转换为对应的*string指针类型
func ToStringPointer(v string) *string {
	return &v
}

// ToStringPointerOrNilIfEmpty 将string类型的变量转换为对应的*string指针类型，如果变量的值为空字符串的话则返回nil指针
func ToStringPointerOrNilIfEmpty(v string) *string {
	if v == "" {
		return nil
	}
	return &v
}

// FromStringPointer 获取*string类型的指针的实际值，如果指针为nil的话则返回空字符串
func FromStringPointer(p *string) string {
	return FromStringPointerOrDefaultIfNil(p, "")
}

// FromStringPointerOrDefaultIfNil 获取*string类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromStringPointerOrDefaultIfNil(v *string, defaultValue string) string {
	if v == nil {
		return defaultValue
	}
	return *v
}
