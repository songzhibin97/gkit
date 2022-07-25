package pointer

// ToBytePointer 将byte类型的变量转换为对应的*byte指针类型
func ToBytePointer(v byte) *byte {
	return &v
}

// ToBytePointerOrNilIfZero 将byte类型的变量转换为对应的*byte指针类型，如果变量的值为0的话则返回nil指针
func ToBytePointerOrNilIfZero(v byte) *byte {
	if v == 0 {
		return nil
	}
	return &v
}

// FromBytePointer 获取*byte类型的指针的实际值，如果指针为nil的话则返回0
func FromBytePointer(p *byte) byte {
	return FromBytePointerOrDefaultIfNil(p, 0)
}

// FromBytePointerOrDefaultIfNil 获取*byte类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromBytePointerOrDefaultIfNil(v *byte, defaultValue byte) byte {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToComplex64Pointer 将complex64类型的变量转换为对应的*complex64指针类型
func ToComplex64Pointer(v complex64) *complex64 {
	return &v
}

// ToComplex64PointerOrNilIfZero 将complex64类型的变量转换为对应的*complex64指针类型，如果变量的值为0的话则返回nil指针
func ToComplex64PointerOrNilIfZero(v complex64) *complex64 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromComplex64Pointer 获取*complex64类型的指针的实际值，如果指针为nil的话则返回0
func FromComplex64Pointer(p *complex64) complex64 {
	return FromComplex64PointerOrDefaultIfNil(p, 0)
}

// FromComplex64PointerOrDefaultIfNil 获取*complex64类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromComplex64PointerOrDefaultIfNil(v *complex64, defaultValue complex64) complex64 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToComplex128Pointer 将complex128类型的变量转换为对应的*complex128指针类型
func ToComplex128Pointer(v complex128) *complex128 {
	return &v
}

// ToComplex128PointerOrNilIfZero 将complex128类型的变量转换为对应的*complex128指针类型，如果变量的值为0的话则返回nil指针
func ToComplex128PointerOrNilIfZero(v complex128) *complex128 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromComplex128Pointer 获取*complex128类型的指针的实际值，如果指针为nil的话则返回0
func FromComplex128Pointer(p *complex128) complex128 {
	return FromComplex128PointerOrDefaultIfNil(p, 0)
}

// FromComplex128PointerOrDefaultIfNil 获取*complex128类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromComplex128PointerOrDefaultIfNil(v *complex128, defaultValue complex128) complex128 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToFloat32Pointer 将float32类型的变量转换为对应的*float32指针类型
func ToFloat32Pointer(v float32) *float32 {
	return &v
}

// ToFloat32PointerOrNilIfZero 将float32类型的变量转换为对应的*float32指针类型，如果变量的值为0的话则返回nil指针
func ToFloat32PointerOrNilIfZero(v float32) *float32 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromFloat32Pointer 获取*float32类型的指针的实际值，如果指针为nil的话则返回0
func FromFloat32Pointer(p *float32) float32 {
	return FromFloat32PointerOrDefaultIfNil(p, 0)
}

// FromFloat32PointerOrDefaultIfNil 获取*float32类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromFloat32PointerOrDefaultIfNil(v *float32, defaultValue float32) float32 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToFloat64Pointer 将float64类型的变量转换为对应的*float64指针类型
func ToFloat64Pointer(v float64) *float64 {
	return &v
}

// ToFloat64PointerOrNilIfZero 将float64类型的变量转换为对应的*float64指针类型，如果变量的值为0的话则返回nil指针
func ToFloat64PointerOrNilIfZero(v float64) *float64 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromFloat64Pointer 获取*float64类型的指针的实际值，如果指针为nil的话则返回0
func FromFloat64Pointer(p *float64) float64 {
	return FromFloat64PointerOrDefaultIfNil(p, 0)
}

// FromFloat64PointerOrDefaultIfNil 获取*float64类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromFloat64PointerOrDefaultIfNil(v *float64, defaultValue float64) float64 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToIntPointer 将int类型的变量转换为对应的*int指针类型
func ToIntPointer(v int) *int {
	return &v
}

// ToIntPointerOrNilIfZero 将int类型的变量转换为对应的*int指针类型，如果变量的值为0的话则返回nil指针
func ToIntPointerOrNilIfZero(v int) *int {
	if v == 0 {
		return nil
	}
	return &v
}

// FromIntPointer 获取*int类型的指针的实际值，如果指针为nil的话则返回0
func FromIntPointer(p *int) int {
	return FromIntPointerOrDefaultIfNil(p, 0)
}

// FromIntPointerOrDefaultIfNil 获取*int类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromIntPointerOrDefaultIfNil(v *int, defaultValue int) int {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToInt8Pointer 将int8类型的变量转换为对应的*int8指针类型
func ToInt8Pointer(v int8) *int8 {
	return &v
}

// ToInt8PointerOrNilIfZero 将int8类型的变量转换为对应的*int8指针类型，如果变量的值为0的话则返回nil指针
func ToInt8PointerOrNilIfZero(v int8) *int8 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromInt8Pointer 获取*int8类型的指针的实际值，如果指针为nil的话则返回0
func FromInt8Pointer(p *int8) int8 {
	return FromInt8PointerOrDefaultIfNil(p, 0)
}

// FromInt8PointerOrDefaultIfNil 获取*int8类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromInt8PointerOrDefaultIfNil(v *int8, defaultValue int8) int8 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToInt16Pointer 将int16类型的变量转换为对应的*int16指针类型
func ToInt16Pointer(v int16) *int16 {
	return &v
}

// ToInt16PointerOrNilIfZero 将int16类型的变量转换为对应的*int16指针类型，如果变量的值为0的话则返回nil指针
func ToInt16PointerOrNilIfZero(v int16) *int16 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromInt16Pointer 获取*int16类型的指针的实际值，如果指针为nil的话则返回0
func FromInt16Pointer(p *int16) int16 {
	return FromInt16PointerOrDefaultIfNil(p, 0)
}

// FromInt16PointerOrDefaultIfNil 获取*int16类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromInt16PointerOrDefaultIfNil(v *int16, defaultValue int16) int16 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToInt32Pointer 将int32类型的变量转换为对应的*int32指针类型
func ToInt32Pointer(v int32) *int32 {
	return &v
}

// ToInt32PointerOrNilIfZero 将int32类型的变量转换为对应的*int32指针类型，如果变量的值为0的话则返回nil指针
func ToInt32PointerOrNilIfZero(v int32) *int32 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromInt32Pointer 获取*int32类型的指针的实际值，如果指针为nil的话则返回0
func FromInt32Pointer(p *int32) int32 {
	return FromInt32PointerOrDefaultIfNil(p, 0)
}

// FromInt32PointerOrDefaultIfNil 获取*int32类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromInt32PointerOrDefaultIfNil(v *int32, defaultValue int32) int32 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToInt64Pointer 将int64类型的变量转换为对应的*int64指针类型
func ToInt64Pointer(v int64) *int64 {
	return &v
}

// ToInt64PointerOrNilIfZero 将int64类型的变量转换为对应的*int64指针类型，如果变量的值为0的话则返回nil指针
func ToInt64PointerOrNilIfZero(v int64) *int64 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromInt64Pointer 获取*int64类型的指针的实际值，如果指针为nil的话则返回0
func FromInt64Pointer(p *int64) int64 {
	return FromInt64PointerOrDefaultIfNil(p, 0)
}

// FromInt64PointerOrDefaultIfNil 获取*int64类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromInt64PointerOrDefaultIfNil(v *int64, defaultValue int64) int64 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToRunePointer 将rune类型的变量转换为对应的*rune指针类型
func ToRunePointer(v rune) *rune {
	return &v
}

// ToRunePointerOrNilIfZero 将rune类型的变量转换为对应的*rune指针类型，如果变量的值为0的话则返回nil指针
func ToRunePointerOrNilIfZero(v rune) *rune {
	if v == 0 {
		return nil
	}
	return &v
}

// FromRunePointer 获取*rune类型的指针的实际值，如果指针为nil的话则返回0
func FromRunePointer(p *rune) rune {
	return FromRunePointerOrDefaultIfNil(p, 0)
}

// FromRunePointerOrDefaultIfNil 获取*rune类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromRunePointerOrDefaultIfNil(v *rune, defaultValue rune) rune {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUintPointer 将uint类型的变量转换为对应的*uint指针类型
func ToUintPointer(v uint) *uint {
	return &v
}

// ToUintPointerOrNilIfZero 将uint类型的变量转换为对应的*uint指针类型，如果变量的值为0的话则返回nil指针
func ToUintPointerOrNilIfZero(v uint) *uint {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUintPointer 获取*uint类型的指针的实际值，如果指针为nil的话则返回0
func FromUintPointer(p *uint) uint {
	return FromUintPointerOrDefaultIfNil(p, 0)
}

// FromUintPointerOrDefaultIfNil 获取*uint类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUintPointerOrDefaultIfNil(v *uint, defaultValue uint) uint {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUint8Pointer 将uint8类型的变量转换为对应的*uint8指针类型
func ToUint8Pointer(v uint8) *uint8 {
	return &v
}

// ToUint8PointerOrNilIfZero 将uint8类型的变量转换为对应的*uint8指针类型，如果变量的值为0的话则返回nil指针
func ToUint8PointerOrNilIfZero(v uint8) *uint8 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUint8Pointer 获取*uint8类型的指针的实际值，如果指针为nil的话则返回0
func FromUint8Pointer(p *uint8) uint8 {
	return FromUint8PointerOrDefaultIfNil(p, 0)
}

// FromUint8PointerOrDefaultIfNil 获取*uint8类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUint8PointerOrDefaultIfNil(v *uint8, defaultValue uint8) uint8 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUint16Pointer 将uint16类型的变量转换为对应的*uint16指针类型
func ToUint16Pointer(v uint16) *uint16 {
	return &v
}

// ToUint16PointerOrNilIfZero 将uint16类型的变量转换为对应的*uint16指针类型，如果变量的值为0的话则返回nil指针
func ToUint16PointerOrNilIfZero(v uint16) *uint16 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUint16Pointer 获取*uint16类型的指针的实际值，如果指针为nil的话则返回0
func FromUint16Pointer(p *uint16) uint16 {
	return FromUint16PointerOrDefaultIfNil(p, 0)
}

// FromUint16PointerOrDefaultIfNil 获取*uint16类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUint16PointerOrDefaultIfNil(v *uint16, defaultValue uint16) uint16 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUint32Pointer 将uint32类型的变量转换为对应的*uint32指针类型
func ToUint32Pointer(v uint32) *uint32 {
	return &v
}

// ToUint32PointerOrNilIfZero 将uint32类型的变量转换为对应的*uint32指针类型，如果变量的值为0的话则返回nil指针
func ToUint32PointerOrNilIfZero(v uint32) *uint32 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUint32Pointer 获取*uint32类型的指针的实际值，如果指针为nil的话则返回0
func FromUint32Pointer(p *uint32) uint32 {
	return FromUint32PointerOrDefaultIfNil(p, 0)
}

// FromUint32PointerOrDefaultIfNil 获取*uint32类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUint32PointerOrDefaultIfNil(v *uint32, defaultValue uint32) uint32 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUint64Pointer 将uint64类型的变量转换为对应的*uint64指针类型
func ToUint64Pointer(v uint64) *uint64 {
	return &v
}

// ToUint64PointerOrNilIfZero 将uint64类型的变量转换为对应的*uint64指针类型，如果变量的值为0的话则返回nil指针
func ToUint64PointerOrNilIfZero(v uint64) *uint64 {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUint64Pointer 获取*uint64类型的指针的实际值，如果指针为nil的话则返回0
func FromUint64Pointer(p *uint64) uint64 {
	return FromUint64PointerOrDefaultIfNil(p, 0)
}

// FromUint64PointerOrDefaultIfNil 获取*uint64类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUint64PointerOrDefaultIfNil(v *uint64, defaultValue uint64) uint64 {
	if v == nil {
		return defaultValue
	}
	return *v
}

// ToUintptrPointer 将uintptr类型的变量转换为对应的*uintptr指针类型
func ToUintptrPointer(v uintptr) *uintptr {
	return &v
}

// ToUintptrPointerOrNilIfZero 将uintptr类型的变量转换为对应的*uintptr指针类型，如果变量的值为0的话则返回nil指针
func ToUintptrPointerOrNilIfZero(v uintptr) *uintptr {
	if v == 0 {
		return nil
	}
	return &v
}

// FromUintptrPointer 获取*uintptr类型的指针的实际值，如果指针为nil的话则返回0
func FromUintptrPointer(p *uintptr) uintptr {
	return FromUintptrPointerOrDefaultIfNil(p, 0)
}

// FromUintptrPointerOrDefaultIfNil 获取*uintptr类型的指针的实际值，如果指针为nil的话则返回defaultValue
func FromUintptrPointerOrDefaultIfNil(v *uintptr, defaultValue uintptr) uintptr {
	if v == nil {
		return defaultValue
	}
	return *v
}
