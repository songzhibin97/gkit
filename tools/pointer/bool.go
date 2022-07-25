package pointer

// ToBoolPointer 将布尔变量转换为布尔指针
func ToBoolPointer(boolValue bool) *bool {
	return &boolValue
}

// ToBoolPointerOrNilIfFalse 将布尔变量转换为布尔类型的指针，如果变量的值为false的话则转换为nil指针
func ToBoolPointerOrNilIfFalse(boolValue bool) *bool {
	if boolValue {
		return &boolValue
	}
	return nil
}

// FromBoolPointer 获取布尔指针实际指向的值，如果指针为nil的话则返回false
func FromBoolPointer(boolPointer *bool) bool {
	return FromBoolPointerOrDefault(boolPointer, false)
}

// FromBoolPointerOrDefault 获取布尔指针实际指向的值，如果指针为nil的话则返回defaultValue
func FromBoolPointerOrDefault(boolPointer *bool, defaultValue bool) bool {
	if boolPointer == nil {
		return defaultValue
	} else {
		return *boolPointer
	}
}
