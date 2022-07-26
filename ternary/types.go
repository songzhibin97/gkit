package ternary

import "time"

// ReturnBool
//  @Description: if实现的三元表达式，返回结果是bool
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的bool
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的bool
//  @return bool: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBool(boolExpression bool, trueReturnValue, falseReturnValue bool) bool {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnBoolSlice
//  @Description: if实现的三元表达式，返回结果是[]bool
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]bool
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]bool
//  @return []bool: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBoolSlice(boolExpression bool, trueReturnValue, falseReturnValue []bool) []bool {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnBoolPointer
//  @Description: if实现的三元表达式，返回结果是*bool
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*bool
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*bool
//  @return *bool: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBoolPointer(boolExpression bool, trueReturnValue, falseReturnValue *bool) *bool {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnBoolPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*bool
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*bool
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*bool
//  @return []*bool: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBoolPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*bool) []*bool {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnByte
//  @Description: if实现的三元表达式，返回结果是byte
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的byte
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的byte
//  @return byte: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnByte(boolExpression bool, trueReturnValue, falseReturnValue byte) byte {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnByteSlice
//  @Description: if实现的三元表达式，返回结果是[]byte
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]byte
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]byte
//  @return []byte: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnByteSlice(boolExpression bool, trueReturnValue, falseReturnValue []byte) []byte {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnBytePointer
//  @Description: if实现的三元表达式，返回结果是*byte
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*byte
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*byte
//  @return *byte: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBytePointer(boolExpression bool, trueReturnValue, falseReturnValue *byte) *byte {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnBytePointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*byte
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*byte
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*byte
//  @return []*byte: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnBytePointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*byte) []*byte {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex64
//  @Description: if实现的三元表达式，返回结果是complex64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的complex64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的complex64
//  @return complex64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex64(boolExpression bool, trueReturnValue, falseReturnValue complex64) complex64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex64Slice
//  @Description: if实现的三元表达式，返回结果是[]complex64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]complex64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]complex64
//  @return []complex64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex64Slice(boolExpression bool, trueReturnValue, falseReturnValue []complex64) []complex64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex64Pointer
//  @Description: if实现的三元表达式，返回结果是*complex64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*complex64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*complex64
//  @return *complex64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex64Pointer(boolExpression bool, trueReturnValue, falseReturnValue *complex64) *complex64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex64PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*complex64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*complex64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*complex64
//  @return []*complex64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex64PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*complex64) []*complex64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex128
//  @Description: if实现的三元表达式，返回结果是complex128
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的complex128
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的complex128
//  @return complex128: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex128(boolExpression bool, trueReturnValue, falseReturnValue complex128) complex128 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex128Slice
//  @Description: if实现的三元表达式，返回结果是[]complex128
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]complex128
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]complex128
//  @return []complex128: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex128Slice(boolExpression bool, trueReturnValue, falseReturnValue []complex128) []complex128 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex128Pointer
//  @Description: if实现的三元表达式，返回结果是*complex128
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*complex128
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*complex128
//  @return *complex128: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex128Pointer(boolExpression bool, trueReturnValue, falseReturnValue *complex128) *complex128 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnComplex128PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*complex128
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*complex128
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*complex128
//  @return []*complex128: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnComplex128PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*complex128) []*complex128 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat32
//  @Description: if实现的三元表达式，返回结果是float32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的float32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的float32
//  @return float32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat32(boolExpression bool, trueReturnValue, falseReturnValue float32) float32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat32Slice
//  @Description: if实现的三元表达式，返回结果是[]float32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]float32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]float32
//  @return []float32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat32Slice(boolExpression bool, trueReturnValue, falseReturnValue []float32) []float32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat32Pointer
//  @Description: if实现的三元表达式，返回结果是*float32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*float32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*float32
//  @return *float32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat32Pointer(boolExpression bool, trueReturnValue, falseReturnValue *float32) *float32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat32PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*float32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*float32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*float32
//  @return []*float32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat32PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*float32) []*float32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat64
//  @Description: if实现的三元表达式，返回结果是float64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的float64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的float64
//  @return float64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat64(boolExpression bool, trueReturnValue, falseReturnValue float64) float64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat64Slice
//  @Description: if实现的三元表达式，返回结果是[]float64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]float64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]float64
//  @return []float64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat64Slice(boolExpression bool, trueReturnValue, falseReturnValue []float64) []float64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat64Pointer
//  @Description: if实现的三元表达式，返回结果是*float64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*float64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*float64
//  @return *float64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat64Pointer(boolExpression bool, trueReturnValue, falseReturnValue *float64) *float64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnFloat64PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*float64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*float64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*float64
//  @return []*float64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnFloat64PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*float64) []*float64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt
//  @Description: if实现的三元表达式，返回结果是int
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的int
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的int
//  @return int: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt(boolExpression bool, trueReturnValue, falseReturnValue int) int {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnIntSlice
//  @Description: if实现的三元表达式，返回结果是[]int
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]int
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]int
//  @return []int: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnIntSlice(boolExpression bool, trueReturnValue, falseReturnValue []int) []int {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnIntPointer
//  @Description: if实现的三元表达式，返回结果是*int
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*int
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*int
//  @return *int: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnIntPointer(boolExpression bool, trueReturnValue, falseReturnValue *int) *int {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnIntPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*int
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*int
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*int
//  @return []*int: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnIntPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*int) []*int {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt8
//  @Description: if实现的三元表达式，返回结果是int8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的int8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的int8
//  @return int8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt8(boolExpression bool, trueReturnValue, falseReturnValue int8) int8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt8Slice
//  @Description: if实现的三元表达式，返回结果是[]int8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]int8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]int8
//  @return []int8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt8Slice(boolExpression bool, trueReturnValue, falseReturnValue []int8) []int8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt8Pointer
//  @Description: if实现的三元表达式，返回结果是*int8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*int8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*int8
//  @return *int8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt8Pointer(boolExpression bool, trueReturnValue, falseReturnValue *int8) *int8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt8PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*int8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*int8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*int8
//  @return []*int8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt8PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*int8) []*int8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt16
//  @Description: if实现的三元表达式，返回结果是int16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的int16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的int16
//  @return int16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt16(boolExpression bool, trueReturnValue, falseReturnValue int16) int16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt16Slice
//  @Description: if实现的三元表达式，返回结果是[]int16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]int16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]int16
//  @return []int16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt16Slice(boolExpression bool, trueReturnValue, falseReturnValue []int16) []int16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt16Pointer
//  @Description: if实现的三元表达式，返回结果是*int16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*int16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*int16
//  @return *int16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt16Pointer(boolExpression bool, trueReturnValue, falseReturnValue *int16) *int16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt16PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*int16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*int16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*int16
//  @return []*int16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt16PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*int16) []*int16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt32
//  @Description: if实现的三元表达式，返回结果是int32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的int32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的int32
//  @return int32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt32(boolExpression bool, trueReturnValue, falseReturnValue int32) int32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt32Slice
//  @Description: if实现的三元表达式，返回结果是[]int32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]int32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]int32
//  @return []int32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt32Slice(boolExpression bool, trueReturnValue, falseReturnValue []int32) []int32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt32Pointer
//  @Description: if实现的三元表达式，返回结果是*int32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*int32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*int32
//  @return *int32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt32Pointer(boolExpression bool, trueReturnValue, falseReturnValue *int32) *int32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt32PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*int32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*int32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*int32
//  @return []*int32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt32PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*int32) []*int32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt64
//  @Description: if实现的三元表达式，返回结果是int64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的int64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的int64
//  @return int64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt64(boolExpression bool, trueReturnValue, falseReturnValue int64) int64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt64Slice
//  @Description: if实现的三元表达式，返回结果是[]int64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]int64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]int64
//  @return []int64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt64Slice(boolExpression bool, trueReturnValue, falseReturnValue []int64) []int64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt64Pointer
//  @Description: if实现的三元表达式，返回结果是*int64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*int64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*int64
//  @return *int64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt64Pointer(boolExpression bool, trueReturnValue, falseReturnValue *int64) *int64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInt64PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*int64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*int64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*int64
//  @return []*int64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInt64PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*int64) []*int64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnRune
//  @Description: if实现的三元表达式，返回结果是rune
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的rune
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的rune
//  @return rune: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnRune(boolExpression bool, trueReturnValue, falseReturnValue rune) rune {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnRuneSlice
//  @Description: if实现的三元表达式，返回结果是[]rune
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]rune
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]rune
//  @return []rune: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnRuneSlice(boolExpression bool, trueReturnValue, falseReturnValue []rune) []rune {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnRunePointer
//  @Description: if实现的三元表达式，返回结果是*rune
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*rune
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*rune
//  @return *rune: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnRunePointer(boolExpression bool, trueReturnValue, falseReturnValue *rune) *rune {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnRunePointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*rune
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*rune
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*rune
//  @return []*rune: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnRunePointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*rune) []*rune {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnString
//  @Description: if实现的三元表达式，返回结果是string
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的string
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的string
//  @return string: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnString(boolExpression bool, trueReturnValue, falseReturnValue string) string {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnStringSlice
//  @Description: if实现的三元表达式，返回结果是[]string
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]string
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]string
//  @return []string: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnStringSlice(boolExpression bool, trueReturnValue, falseReturnValue []string) []string {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnStringPointer
//  @Description: if实现的三元表达式，返回结果是*string
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*string
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*string
//  @return *string: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnStringPointer(boolExpression bool, trueReturnValue, falseReturnValue *string) *string {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnStringPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*string
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*string
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*string
//  @return []*string: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnStringPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*string) []*string {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint
//  @Description: if实现的三元表达式，返回结果是uint
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uint
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uint
//  @return uint: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint(boolExpression bool, trueReturnValue, falseReturnValue uint) uint {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintSlice
//  @Description: if实现的三元表达式，返回结果是[]uint
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uint
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uint
//  @return []uint: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintSlice(boolExpression bool, trueReturnValue, falseReturnValue []uint) []uint {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintPointer
//  @Description: if实现的三元表达式，返回结果是*uint
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uint
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uint
//  @return *uint: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintPointer(boolExpression bool, trueReturnValue, falseReturnValue *uint) *uint {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uint
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uint
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uint
//  @return []*uint: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uint) []*uint {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint8
//  @Description: if实现的三元表达式，返回结果是uint8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uint8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uint8
//  @return uint8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint8(boolExpression bool, trueReturnValue, falseReturnValue uint8) uint8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint8Slice
//  @Description: if实现的三元表达式，返回结果是[]uint8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uint8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uint8
//  @return []uint8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint8Slice(boolExpression bool, trueReturnValue, falseReturnValue []uint8) []uint8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint8Pointer
//  @Description: if实现的三元表达式，返回结果是*uint8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uint8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uint8
//  @return *uint8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint8Pointer(boolExpression bool, trueReturnValue, falseReturnValue *uint8) *uint8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint8PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uint8
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uint8
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uint8
//  @return []*uint8: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint8PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uint8) []*uint8 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint16
//  @Description: if实现的三元表达式，返回结果是uint16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uint16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uint16
//  @return uint16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint16(boolExpression bool, trueReturnValue, falseReturnValue uint16) uint16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint16Slice
//  @Description: if实现的三元表达式，返回结果是[]uint16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uint16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uint16
//  @return []uint16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint16Slice(boolExpression bool, trueReturnValue, falseReturnValue []uint16) []uint16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint16Pointer
//  @Description: if实现的三元表达式，返回结果是*uint16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uint16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uint16
//  @return *uint16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint16Pointer(boolExpression bool, trueReturnValue, falseReturnValue *uint16) *uint16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint16PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uint16
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uint16
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uint16
//  @return []*uint16: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint16PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uint16) []*uint16 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint32
//  @Description: if实现的三元表达式，返回结果是uint32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uint32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uint32
//  @return uint32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint32(boolExpression bool, trueReturnValue, falseReturnValue uint32) uint32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint32Slice
//  @Description: if实现的三元表达式，返回结果是[]uint32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uint32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uint32
//  @return []uint32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint32Slice(boolExpression bool, trueReturnValue, falseReturnValue []uint32) []uint32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint32Pointer
//  @Description: if实现的三元表达式，返回结果是*uint32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uint32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uint32
//  @return *uint32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint32Pointer(boolExpression bool, trueReturnValue, falseReturnValue *uint32) *uint32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint32PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uint32
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uint32
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uint32
//  @return []*uint32: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint32PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uint32) []*uint32 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint64
//  @Description: if实现的三元表达式，返回结果是uint64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uint64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uint64
//  @return uint64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint64(boolExpression bool, trueReturnValue, falseReturnValue uint64) uint64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint64Slice
//  @Description: if实现的三元表达式，返回结果是[]uint64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uint64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uint64
//  @return []uint64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint64Slice(boolExpression bool, trueReturnValue, falseReturnValue []uint64) []uint64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint64Pointer
//  @Description: if实现的三元表达式，返回结果是*uint64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uint64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uint64
//  @return *uint64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint64Pointer(boolExpression bool, trueReturnValue, falseReturnValue *uint64) *uint64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUint64PointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uint64
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uint64
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uint64
//  @return []*uint64: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUint64PointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uint64) []*uint64 {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintptr
//  @Description: if实现的三元表达式，返回结果是uintptr
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的uintptr
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的uintptr
//  @return uintptr: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintptr(boolExpression bool, trueReturnValue, falseReturnValue uintptr) uintptr {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintptrSlice
//  @Description: if实现的三元表达式，返回结果是[]uintptr
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]uintptr
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]uintptr
//  @return []uintptr: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintptrSlice(boolExpression bool, trueReturnValue, falseReturnValue []uintptr) []uintptr {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintptrPointer
//  @Description: if实现的三元表达式，返回结果是*uintptr
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*uintptr
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*uintptr
//  @return *uintptr: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintptrPointer(boolExpression bool, trueReturnValue, falseReturnValue *uintptr) *uintptr {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnUintptrPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*uintptr
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*uintptr
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*uintptr
//  @return []*uintptr: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnUintptrPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*uintptr) []*uintptr {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInterface
//  @Description: if实现的三元表达式，返回结果是interface{}
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的interface{}
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的interface{}
//  @return interface{}: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInterface(boolExpression bool, trueReturnValue, falseReturnValue interface{}) interface{} {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInterfaceSlice
//  @Description: if实现的三元表达式，返回结果是[]interface{}
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]interface{}
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]interface{}
//  @return []interface{}: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInterfaceSlice(boolExpression bool, trueReturnValue, falseReturnValue []interface{}) []interface{} {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInterfacePointer
//  @Description: if实现的三元表达式，返回结果是*interface{}
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*interface{}
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*interface{}
//  @return *interface{}: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInterfacePointer(boolExpression bool, trueReturnValue, falseReturnValue *interface{}) *interface{} {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnInterfacePointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*interface{}
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*interface{}
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*interface{}
//  @return []*interface{}: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnInterfacePointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*interface{}) []*interface{} {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnTime
//  @Description: if实现的三元表达式，返回结果是time.Time
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的time.Time
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的time.Time
//  @return time.Time: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnTime(boolExpression bool, trueReturnValue, falseReturnValue time.Time) time.Time {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnTimeSlice
//  @Description: if实现的三元表达式，返回结果是[]time.Time
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]time.Time
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]time.Time
//  @return []time.Time: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnTimeSlice(boolExpression bool, trueReturnValue, falseReturnValue []time.Time) []time.Time {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnTimePointer
//  @Description: if实现的三元表达式，返回结果是*time.Time
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*time.Time
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*time.Time
//  @return *time.Time: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnTimePointer(boolExpression bool, trueReturnValue, falseReturnValue *time.Time) *time.Time {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnTimePointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*time.Time
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*time.Time
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*time.Time
//  @return []*time.Time: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnTimePointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*time.Time) []*time.Time {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnDuration
//  @Description: if实现的三元表达式，返回结果是time.Duration
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的time.Duration
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的time.Duration
//  @return time.Duration: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnDuration(boolExpression bool, trueReturnValue, falseReturnValue time.Duration) time.Duration {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnDurationSlice
//  @Description: if实现的三元表达式，返回结果是[]time.Duration
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]time.Duration
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]time.Duration
//  @return []time.Duration: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnDurationSlice(boolExpression bool, trueReturnValue, falseReturnValue []time.Duration) []time.Duration {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnDurationPointer
//  @Description: if实现的三元表达式，返回结果是*time.Duration
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的*time.Duration
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的*time.Duration
//  @return *time.Duration: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnDurationPointer(boolExpression bool, trueReturnValue, falseReturnValue *time.Duration) *time.Duration {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}

// ReturnDurationPointerSlice
//  @Description: if实现的三元表达式，返回结果是[]*time.Duration
//  @param boolExpression: 表达式，最终返回一个布尔值
//  @param trueReturnValue: 当boolExpression返回值为true的时候返回的[]*time.Duration
//  @param falseReturnValue: 当boolExpression返回值为false的时候返回的[]*time.Duration
//  @return []*time.Duration: 三元表达式的结果，为trueReturnValue或者falseReturnValue中的一个
func ReturnDurationPointerSlice(boolExpression bool, trueReturnValue, falseReturnValue []*time.Duration) []*time.Duration {
	if boolExpression {
		return trueReturnValue
	} else {
		return falseReturnValue
	}
}
