package local_cache

import (
	"bytes"
	"io/ioutil"
	"runtime"
	"strconv"
	"sync"
	"testing"
	"time"
)

type TestStruct struct {
	Num      int
	Children []*TestStruct
}

func TestCache(t *testing.T) {
	tc := NewCache()

	a, found := tc.Get("a")
	if found || a != nil {
		t.Error("a exist:", a)
	}

	b, found := tc.Get("b")
	if found || b != nil {
		t.Error("b exist:", b)
	}

	c, found := tc.Get("c")
	if found || c != nil {
		t.Error("c exist::", c)
	}

	tc.Set("a", 1, DefaultExpire)
	tc.Set("b", "b", DefaultExpire)
	tc.Set("c", 3.5, DefaultExpire)

	v, found := tc.Get("a")
	if !found {
		t.Error("a not exist")
	}
	if v == nil {
		t.Error("a == nil")
	} else if vv := v.(int); vv+2 != 3 {
		t.Error("vv != 3", vv)
	}

	v, found = tc.Get("b")
	if !found {
		t.Error("b not exist")
	}
	if v == nil {
		t.Error("b == nil")
	} else if vv := v.(string); vv+"B" != "bB" {
		t.Error("bb != bB:", vv)
	}

	v, found = tc.Get("c")
	if !found {
		t.Error("c not exist")
	}
	if v == nil {
		t.Error("x for c is nil")
	} else if vv := v.(float64); vv+1.2 != 4.7 {
		t.Error("vv != 4,7:", vv)
	}
}

func TestCacheTimes(t *testing.T) {
	var found bool
	tc := NewCache(SetDefaultExpire(50*time.Millisecond), SetInternal(time.Millisecond))
	tc.Set("a", 1, DefaultExpire)
	tc.Set("b", 2, NoExpire)
	tc.Set("c", 3, 20*time.Millisecond)
	tc.Set("d", 4, 70*time.Millisecond)

	<-time.After(25 * time.Millisecond)
	_, found = tc.Get("c")
	if found {
		t.Error("Found c when it should have been automatically deleted")
	}

	<-time.After(30 * time.Millisecond)
	_, found = tc.Get("a")
	if found {
		t.Error("Found a when it should have been automatically deleted")
	}

	_, found = tc.Get("b")
	if !found {
		t.Error("Did not find b even though it was set to never expire")
	}

	_, found = tc.Get("d")
	if !found {
		t.Error("Did not find d even though it was set to expire later than the default")
	}

	<-time.After(20 * time.Millisecond)
	_, found = tc.Get("d")
	if found {
		t.Error("Found d when it should have been automatically deleted (later than the default)")
	}
}

func TestNewFrom(t *testing.T) {
	m := map[string]Iterator{
		"a": {Val: 1, Expire: 0},
		"b": {Val: 2, Expire: 0},
	}
	tc := NewCache(SetInternal(0), SetMember(m))
	a, found := tc.Get("a")
	if !found {
		t.Fatal("Did not find a")
	}
	if a.(int) != 1 {
		t.Fatal("a is not 1")
	}
	b, found := tc.Get("b")
	if !found {
		t.Fatal("Did not find b")
	}
	if b.(int) != 2 {
		t.Fatal("b is not 2")
	}
}

func TestStorePointerToStruct(t *testing.T) {
	tc := NewCache()
	tc.Set("foo", &TestStruct{Num: 1}, DefaultExpire)
	x, found := tc.Get("foo")
	if !found {
		t.Fatal("*TestStruct was not found for foo")
	}
	foo := x.(*TestStruct)
	foo.Num++

	y, found := tc.Get("foo")
	if !found {
		t.Fatal("*TestStruct was not found for foo (second time)")
	}
	bar := y.(*TestStruct)
	if bar.Num != 2 {
		t.Fatal("TestStruct.Num is not 2")
	}
}

func TestIncrementWithInt(t *testing.T) {
	tc := NewCache()
	tc.Set("tint", 1, DefaultExpire)
	err := tc.Increment("tint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint")
	if !found {
		t.Error("tint was not found")
	}
	if x.(int) != 3 {
		t.Error("tint is not 3:", x)
	}
}

func TestIncrementWithInt8(t *testing.T) {
	tc := NewCache()
	tc.Set("tint8", int8(1), DefaultExpire)
	err := tc.Increment("tint8", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint8")
	if !found {
		t.Error("tint8 was not found")
	}
	if x.(int8) != 3 {
		t.Error("tint8 is not 3:", x)
	}
}

func TestIncrementWithInt16(t *testing.T) {
	tc := NewCache()
	tc.Set("tint16", int16(1), DefaultExpire)
	err := tc.Increment("tint16", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint16")
	if !found {
		t.Error("tint16 was not found")
	}
	if x.(int16) != 3 {
		t.Error("tint16 is not 3:", x)
	}
}

func TestIncrementWithInt32(t *testing.T) {
	tc := NewCache()
	tc.Set("tint32", int32(1), DefaultExpire)
	err := tc.Increment("tint32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint32")
	if !found {
		t.Error("tint32 was not found")
	}
	if x.(int32) != 3 {
		t.Error("tint32 is not 3:", x)
	}
}

func TestIncrementWithInt64(t *testing.T) {
	tc := NewCache()
	tc.Set("tint64", int64(1), DefaultExpire)
	err := tc.Increment("tint64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tint64")
	if !found {
		t.Error("tint64 was not found")
	}
	if x.(int64) != 3 {
		t.Error("tint64 is not 3:", x)
	}
}

func TestIncrementWithUint(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint", uint(1), DefaultExpire)
	err := tc.Increment("tUint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tUint")
	if !found {
		t.Error("tUint was not found")
	}
	if x.(uint) != 3 {
		t.Error("tUint is not 3:", x)
	}
}

func TestIncrementWithUintPtr(t *testing.T) {
	tc := NewCache()
	tc.Set("tUintPtr", uintptr(1), DefaultExpire)
	err := tc.Increment("tUintPtr", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}

	x, found := tc.Get("tUintPtr")
	if !found {
		t.Error("tUintPtr was not found")
	}
	if x.(uintptr) != 3 {
		t.Error("tUintPtr is not 3:", x)
	}
}

func TestIncrementWithUint8(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint8", uint8(1), DefaultExpire)
	err := tc.Increment("tUint8", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tUint8")
	if !found {
		t.Error("tUint8 was not found")
	}
	if x.(uint8) != 3 {
		t.Error("tUint8 is not 3:", x)
	}
}

func TestIncrementWithUint16(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint16", uint16(1), DefaultExpire)
	err := tc.Increment("tUint16", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}

	x, found := tc.Get("tUint16")
	if !found {
		t.Error("tUint16 was not found")
	}
	if x.(uint16) != 3 {
		t.Error("tUint16 is not 3:", x)
	}
}

func TestIncrementWithUint32(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint32", uint32(1), DefaultExpire)
	err := tc.Increment("tUint32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("tUint32")
	if !found {
		t.Error("tUint32 was not found")
	}
	if x.(uint32) != 3 {
		t.Error("tUint32 is not 3:", x)
	}
}

func TestIncrementWithUint64(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint64", uint64(1), DefaultExpire)
	err := tc.Increment("tUint64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}

	x, found := tc.Get("tUint64")
	if !found {
		t.Error("tUint64 was not found")
	}
	if x.(uint64) != 3 {
		t.Error("tUint64 is not 3:", x)
	}
}

func TestIncrementWithFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(1.5), DefaultExpire)
	err := tc.Increment("float32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3.5 {
		t.Error("float32 is not 3.5:", x)
	}
}

func TestIncrementWithFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", 1.5, DefaultExpire)
	err := tc.Increment("float64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3.5 {
		t.Error("float64 is not 3.5:", x)
	}
}

func TestIncrementFloatWithFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(1.5), DefaultExpire)
	err := tc.IncrementFloat("float32", 2)
	if err != nil {
		t.Error("Error incrementFloating:", err)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3.5 {
		t.Error("float32 is not 3.5:", x)
	}
}

func TestIncrementFloatWithFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", 1.5, DefaultExpire)
	err := tc.IncrementFloat("float64", 2)
	if err != nil {
		t.Error("Error incrementFloating:", err)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3.5 {
		t.Error("float64 is not 3.5:", x)
	}
}

func TestDecrementWithInt(t *testing.T) {
	tc := NewCache()
	tc.Set("int", 5, DefaultExpire)
	err := tc.Decrement("int", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int")
	if !found {
		t.Error("int was not found")
	}
	if x.(int) != 3 {
		t.Error("int is not 3:", x)
	}
}

func TestDecrementWithInt8(t *testing.T) {
	tc := NewCache()
	tc.Set("int8", int8(5), DefaultExpire)
	err := tc.Decrement("int8", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int8")
	if !found {
		t.Error("int8 was not found")
	}
	if x.(int8) != 3 {
		t.Error("int8 is not 3:", x)
	}
}

func TestDecrementWithInt16(t *testing.T) {
	tc := NewCache()
	tc.Set("int16", int16(5), DefaultExpire)
	err := tc.Decrement("int16", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int16")
	if !found {
		t.Error("int16 was not found")
	}
	if x.(int16) != 3 {
		t.Error("int16 is not 3:", x)
	}
}

func TestDecrementWithInt32(t *testing.T) {
	tc := NewCache()
	tc.Set("int32", int32(5), DefaultExpire)
	err := tc.Decrement("int32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int32")
	if !found {
		t.Error("int32 was not found")
	}
	if x.(int32) != 3 {
		t.Error("int32 is not 3:", x)
	}
}

func TestDecrementWithInt64(t *testing.T) {
	tc := NewCache()
	tc.Set("int64", int64(5), DefaultExpire)
	err := tc.Decrement("int64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("int64")
	if !found {
		t.Error("int64 was not found")
	}
	if x.(int64) != 3 {
		t.Error("int64 is not 3:", x)
	}
}

func TestDecrementWithUint(t *testing.T) {
	tc := NewCache()
	tc.Set("uint", uint(5), DefaultExpire)
	err := tc.Decrement("uint", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uint")
	if !found {
		t.Error("uint was not found")
	}
	if x.(uint) != 3 {
		t.Error("uint is not 3:", x)
	}
}

func TestDecrementWithUintPtr(t *testing.T) {
	tc := NewCache()
	tc.Set("uintPtr", uintptr(5), DefaultExpire)
	err := tc.Decrement("uintPtr", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uintPtr")
	if !found {
		t.Error("uintPtr was not found")
	}
	if x.(uintptr) != 3 {
		t.Error("uintPtr is not 3:", x)
	}
}

func TestDecrementWithUint8(t *testing.T) {
	tc := NewCache()
	tc.Set("uint8", uint8(5), DefaultExpire)
	err := tc.Decrement("uint8", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uint8")
	if !found {
		t.Error("uint8 was not found")
	}
	if x.(uint8) != 3 {
		t.Error("uint8 is not 3:", x)
	}
}

func TestDecrementWithUint16(t *testing.T) {
	tc := NewCache()
	tc.Set("uint16", uint16(5), DefaultExpire)
	err := tc.Decrement("uint16", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uint16")
	if !found {
		t.Error("uint16 was not found")
	}
	if x.(uint16) != 3 {
		t.Error("uint16 is not 3:", x)
	}
}

func TestDecrementWithUint32(t *testing.T) {
	tc := NewCache()
	tc.Set("uint32", uint32(5), DefaultExpire)
	err := tc.Decrement("uint32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uint32")
	if !found {
		t.Error("uint32 was not found")
	}
	if x.(uint32) != 3 {
		t.Error("uint32 is not 3:", x)
	}
}

func TestDecrementWithUint64(t *testing.T) {
	tc := NewCache()
	tc.Set("uint64", uint64(5), DefaultExpire)
	err := tc.Decrement("uint64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("uint64")
	if !found {
		t.Error("uint64 was not found")
	}
	if x.(uint64) != 3 {
		t.Error("uint64 is not 3:", x)
	}
}

func TestDecrementWithFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(5.5), DefaultExpire)
	err := tc.Decrement("float32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3.5 {
		t.Error("float32 is not 3:", x)
	}
}

func TestDecrementWithFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", 5.5, DefaultExpire)
	err := tc.Decrement("float64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3.5 {
		t.Error("float64 is not 3:", x)
	}
}

func TestDecrementFloatWithFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(5.5), DefaultExpire)
	err := tc.DecrementFloat("float32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3.5 {
		t.Error("float32 is not 3:", x)
	}
}

func TestDecrementFloatWithFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", 5.5, DefaultExpire)
	err := tc.DecrementFloat("float64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3.5 {
		t.Error("float64 is not 3:", x)
	}
}

func TestIncrementInt(t *testing.T) {
	tc := NewCache()
	tc.Set("tint", 1, DefaultExpire)
	n, err := tc.IncrementInt("tint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint")
	if !found {
		t.Error("tint was not found")
	}
	if x.(int) != 3 {
		t.Error("tint is not 3:", x)
	}
}

func TestIncrementInt8(t *testing.T) {
	tc := NewCache()
	tc.Set("tint8", int8(1), DefaultExpire)
	n, err := tc.IncrementInt8("tint8", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint8")
	if !found {
		t.Error("tint8 was not found")
	}
	if x.(int8) != 3 {
		t.Error("tint8 is not 3:", x)
	}
}

func TestIncrementInt16(t *testing.T) {
	tc := NewCache()
	tc.Set("tint16", int16(1), DefaultExpire)
	n, err := tc.IncrementInt16("tint16", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint16")
	if !found {
		t.Error("tint16 was not found")
	}
	if x.(int16) != 3 {
		t.Error("tint16 is not 3:", x)
	}
}

func TestIncrementInt32(t *testing.T) {
	tc := NewCache()
	tc.Set("tint32", int32(1), DefaultExpire)
	n, err := tc.IncrementInt32("tint32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint32")
	if !found {
		t.Error("tint32 was not found")
	}
	if x.(int32) != 3 {
		t.Error("tint32 is not 3:", x)
	}
}

func TestIncrementInt64(t *testing.T) {
	tc := NewCache()
	tc.Set("tint64", int64(1), DefaultExpire)
	n, err := tc.IncrementInt64("tint64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tint64")
	if !found {
		t.Error("tint64 was not found")
	}
	if x.(int64) != 3 {
		t.Error("tint64 is not 3:", x)
	}
}

func TestIncrementUint(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint", uint(1), DefaultExpire)
	n, err := tc.IncrementUint("tUint", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUint")
	if !found {
		t.Error("tUint was not found")
	}
	if x.(uint) != 3 {
		t.Error("tUint is not 3:", x)
	}
}

func TestIncrementUintPtr(t *testing.T) {
	tc := NewCache()
	tc.Set("tUintPtr", uintptr(1), DefaultExpire)
	n, err := tc.IncrementUintPtr("tUintPtr", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUintPtr")
	if !found {
		t.Error("tUintPtr was not found")
	}
	if x.(uintptr) != 3 {
		t.Error("tUintPtr is not 3:", x)
	}
}

func TestIncrementUint8(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint8", uint8(1), DefaultExpire)
	n, err := tc.IncrementUint8("tUint8", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUint8")
	if !found {
		t.Error("tUint8 was not found")
	}
	if x.(uint8) != 3 {
		t.Error("tUint8 is not 3:", x)
	}
}

func TestIncrementUint16(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint16", uint16(1), DefaultExpire)
	n, err := tc.IncrementUint16("tUint16", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUint16")
	if !found {
		t.Error("tUint16 was not found")
	}
	if x.(uint16) != 3 {
		t.Error("tUint16 is not 3:", x)
	}
}

func TestIncrementUint32(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint32", uint32(1), DefaultExpire)
	n, err := tc.IncrementUint32("tUint32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUint32")
	if !found {
		t.Error("tUint32 was not found")
	}
	if x.(uint32) != 3 {
		t.Error("tUint32 is not 3:", x)
	}
}

func TestIncrementUint64(t *testing.T) {
	tc := NewCache()
	tc.Set("tUint64", uint64(1), DefaultExpire)
	n, err := tc.IncrementUint64("tUint64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("tUint64")
	if !found {
		t.Error("tUint64 was not found")
	}
	if x.(uint64) != 3 {
		t.Error("tUint64 is not 3:", x)
	}
}

func TestIncrementFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(1.5), DefaultExpire)
	n, err := tc.IncrementFloat32("float32", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3.5 {
		t.Error("Returned number is not 3.5:", n)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3.5 {
		t.Error("float32 is not 3.5:", x)
	}
}

func TestIncrementFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", 1.5, DefaultExpire)
	n, err := tc.IncrementFloat64("float64", 2)
	if err != nil {
		t.Error("Error incrementing:", err)
	}
	if n != 3.5 {
		t.Error("Returned number is not 3.5:", n)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3.5 {
		t.Error("float64 is not 3.5:", x)
	}
}

func TestDecrementInt8(t *testing.T) {
	tc := NewCache()
	tc.Set("int8", int8(5), DefaultExpire)
	n, err := tc.DecrementInt8("int8", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("int8")
	if !found {
		t.Error("int8 was not found")
	}
	if x.(int8) != 3 {
		t.Error("int8 is not 3:", x)
	}
}

func TestDecrementInt16(t *testing.T) {
	tc := NewCache()
	tc.Set("int16", int16(5), DefaultExpire)
	n, err := tc.DecrementInt16("int16", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("int16")
	if !found {
		t.Error("int16 was not found")
	}
	if x.(int16) != 3 {
		t.Error("int16 is not 3:", x)
	}
}

func TestDecrementInt32(t *testing.T) {
	tc := NewCache()
	tc.Set("int32", int32(5), DefaultExpire)
	n, err := tc.DecrementInt32("int32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("int32")
	if !found {
		t.Error("int32 was not found")
	}
	if x.(int32) != 3 {
		t.Error("int32 is not 3:", x)
	}
}

func TestDecrementInt64(t *testing.T) {
	tc := NewCache()
	tc.Set("int64", int64(5), DefaultExpire)
	n, err := tc.DecrementInt64("int64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("int64")
	if !found {
		t.Error("int64 was not found")
	}
	if x.(int64) != 3 {
		t.Error("int64 is not 3:", x)
	}
}

func TestDecrementUint(t *testing.T) {
	tc := NewCache()
	tc.Set("uint", uint(5), DefaultExpire)
	n, err := tc.DecrementUint("uint", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uint")
	if !found {
		t.Error("uint was not found")
	}
	if x.(uint) != 3 {
		t.Error("uint is not 3:", x)
	}
}

func TestDecrementUintPtr(t *testing.T) {
	tc := NewCache()
	tc.Set("uintPtr", uintptr(5), DefaultExpire)
	n, err := tc.DecrementUintPtr("uintPtr", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uintPtr")
	if !found {
		t.Error("uintPtr was not found")
	}
	if x.(uintptr) != 3 {
		t.Error("uintPtr is not 3:", x)
	}
}

func TestDecrementUint8(t *testing.T) {
	tc := NewCache()
	tc.Set("uint8", uint8(5), DefaultExpire)
	n, err := tc.DecrementUint8("uint8", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uint8")
	if !found {
		t.Error("uint8 was not found")
	}
	if x.(uint8) != 3 {
		t.Error("uint8 is not 3:", x)
	}
}

func TestDecrementUint16(t *testing.T) {
	tc := NewCache()
	tc.Set("uint16", uint16(5), DefaultExpire)
	n, err := tc.DecrementUint16("uint16", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uint16")
	if !found {
		t.Error("uint16 was not found")
	}
	if x.(uint16) != 3 {
		t.Error("uint16 is not 3:", x)
	}
}

func TestDecrementUint32(t *testing.T) {
	tc := NewCache()
	tc.Set("uint32", uint32(5), DefaultExpire)
	n, err := tc.DecrementUint32("uint32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uint32")
	if !found {
		t.Error("uint32 was not found")
	}
	if x.(uint32) != 3 {
		t.Error("uint32 is not 3:", x)
	}
}

func TestDecrementUint64(t *testing.T) {
	tc := NewCache()
	tc.Set("uint64", uint64(5), DefaultExpire)
	n, err := tc.DecrementUint64("uint64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("uint64")
	if !found {
		t.Error("uint64 was not found")
	}
	if x.(uint64) != 3 {
		t.Error("uint64 is not 3:", x)
	}
}

func TestDecrementFloat32(t *testing.T) {
	tc := NewCache()
	tc.Set("float32", float32(5), DefaultExpire)
	n, err := tc.DecrementFloat32("float32", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("float32")
	if !found {
		t.Error("float32 was not found")
	}
	if x.(float32) != 3 {
		t.Error("float32 is not 3:", x)
	}
}

func TestDecrementFloat64(t *testing.T) {
	tc := NewCache()
	tc.Set("float64", float64(5), DefaultExpire)
	n, err := tc.DecrementFloat64("float64", 2)
	if err != nil {
		t.Error("Error decrementing:", err)
	}
	if n != 3 {
		t.Error("Returned number is not 3:", n)
	}
	x, found := tc.Get("float64")
	if !found {
		t.Error("float64 was not found")
	}
	if x.(float64) != 3 {
		t.Error("float64 is not 3:", x)
	}
}

func TestAdd(t *testing.T) {
	tc := NewCache()
	err := tc.Add("foo", "bar", DefaultExpire)
	if err != nil {
		t.Error("Couldn't add foo even though it shouldn't exist")
	}
	err = tc.Add("foo", "baz", DefaultExpire)
	if err == nil {
		t.Error("Successfully added another foo when it should have returned an error")
	}
}

func TestReplace(t *testing.T) {
	tc := NewCache()
	err := tc.Replace("foo", "bar", DefaultExpire)
	if err == nil {
		t.Error("Replaced foo when it shouldn't exist")
	}
	tc.Set("foo", "bar", DefaultExpire)
	err = tc.Replace("foo", "bar", DefaultExpire)
	if err != nil {
		t.Error("Couldn't replace existing key foo")
	}
}

func TestDelete(t *testing.T) {
	tc := NewCache()
	tc.Set("foo", "bar", DefaultExpire)
	tc.Delete("foo")
	x, found := tc.Get("foo")
	if found {
		t.Error("foo was found, but it should have been deleted")
	}
	if x != nil {
		t.Error("x is not nil:", x)
	}
}

func TestItemCount(t *testing.T) {
	tc := NewCache()
	tc.Set("foo", "1", DefaultExpire)
	tc.Set("bar", "2", DefaultExpire)
	tc.Set("baz", "3", DefaultExpire)
	if n := tc.Count(); n != 3 {
		t.Errorf("Item count is not 3: %d", n)
	}
}

func TestFlush(t *testing.T) {
	tc := NewCache()
	tc.Set("foo", "bar", DefaultExpire)
	tc.Set("baz", "yes", DefaultExpire)
	tc.Flush()
	x, found := tc.Get("foo")
	if found {
		t.Error("foo was found, but it should have been deleted")
	}
	if x != nil {
		t.Error("x is not nil:", x)
	}
	x, found = tc.Get("baz")
	if found {
		t.Error("baz was found, but it should have been deleted")
	}
	if x != nil {
		t.Error("x is not nil:", x)
	}
}

func TestIncrementOverflowInt(t *testing.T) {
	tc := NewCache()
	tc.Set("i8", int8(127), DefaultExpire)
	err := tc.Increment("i8", 1)
	if err != nil {
		t.Error("Error incrementing i8:", err)
	}
	x, _ := tc.Get("i8")
	i8 := x.(int8)
	if i8 != -128 {
		t.Error("i8 did not overflow as expected; value:", i8)
	}
}

func TestIncrementOverflowUint(t *testing.T) {
	tc := NewCache()
	tc.Set("ui8", uint8(255), DefaultExpire)
	err := tc.Increment("ui8", 1)
	if err != nil {
		t.Error("Error incrementing int8:", err)
	}
	x, _ := tc.Get("ui8")
	ui8 := x.(uint8)
	if ui8 != 0 {
		t.Error("ui8 did not overflow as expected; value:", ui8)
	}
}

func TestDecrementUnderflowUint(t *testing.T) {
	tc := NewCache()
	tc.Set("ui8", uint8(0), DefaultExpire)
	err := tc.Decrement("ui8", 1)
	if err != nil {
		t.Error("Error decrementing int8:", err)
	}
	x, _ := tc.Get("ui8")
	ui8 := x.(uint8)
	if ui8 != 255 {
		t.Error("ui8 did not underflow as expected; value:", ui8)
	}
}

func TestOnEvicted(t *testing.T) {
	tc := NewCache()
	tc.Set("foo", 3, DefaultExpire)
	if tc.capture == nil {
		t.Fatal("tc.onEvicted is nil")
	}
	works := false
	tc.ChangeCapture(func(k string, v interface{}) {
		if k == "foo" && v.(int) == 3 {
			works = true
		}
		tc.Set("bar", 4, DefaultExpire)
	})
	tc.Delete("foo")
	x, _ := tc.Get("bar")
	if !works {
		t.Error("works bool not true")
	}
	if x.(int) != 4 {
		t.Error("bar was not 4")
	}
}

func TestCacheSerialization(t *testing.T) {
	tc := NewCache()
	testFillAndSerialize(t, &tc)

	// Check if gob.Register behaves properly even after multiple gob.Register
	// on c.Items (many of which will be the same type)
	testFillAndSerialize(t, &tc)
}

func testFillAndSerialize(t *testing.T, tc *Cache) {
	tc.Set("a", "a", DefaultExpire)
	tc.Set("b", "b", DefaultExpire)
	tc.Set("c", "c", DefaultExpire)
	tc.Set("expired", "foo", 1*time.Millisecond)
	tc.Set("*struct", &TestStruct{Num: 1}, DefaultExpire)
	tc.Set("[]struct", []TestStruct{
		{Num: 2},
		{Num: 3},
	}, DefaultExpire)
	tc.Set("[]*struct", []*TestStruct{
		{Num: 4},
		{Num: 5},
	}, DefaultExpire)
	tc.Set("structuration", &TestStruct{
		Num: 42,
		Children: []*TestStruct{
			{Num: 6174},
			{Num: 4716},
		},
	}, DefaultExpire)

	fp := &bytes.Buffer{}
	err := tc.Save(fp)
	if err != nil {
		t.Fatal("Couldn't save cache to fp:", err)
	}

	oc := NewCache()
	err = oc.Load(fp)
	if err != nil {
		t.Fatal("Couldn't load cache from fp:", err)
	}

	a, found := oc.Get("a")
	if !found {
		t.Error("a was not found")
	}
	if a.(string) != "a" {
		t.Error("a is not a")
	}

	b, found := oc.Get("b")
	if !found {
		t.Error("b was not found")
	}
	if b.(string) != "b" {
		t.Error("b is not b")
	}

	c, found := oc.Get("c")
	if !found {
		t.Error("c was not found")
	}
	if c.(string) != "c" {
		t.Error("c is not c")
	}

	<-time.After(5 * time.Millisecond)
	_, found = oc.Get("expired")
	if found {
		t.Error("expired was found")
	}

	s1, found := oc.Get("*struct")
	if !found {
		t.Error("*struct was not found")
	}
	if s1.(*TestStruct).Num != 1 {
		t.Error("*struct.Num is not 1")
	}

	s2, found := oc.Get("[]struct")
	if !found {
		t.Error("[]struct was not found")
	}
	s2r := s2.([]TestStruct)
	if len(s2r) != 2 {
		t.Error("Length of s2r is not 2")
	}
	if s2r[0].Num != 2 {
		t.Error("s2r[0].Num is not 2")
	}
	if s2r[1].Num != 3 {
		t.Error("s2r[1].Num is not 3")
	}

	s3, found := oc.get("[]*struct")
	if !found {
		t.Error("[]*struct was not found")
	}
	s3r := s3.([]*TestStruct)
	if len(s3r) != 2 {
		t.Error("Length of s3r is not 2")
	}
	if s3r[0].Num != 4 {
		t.Error("s3r[0].Num is not 4")
	}
	if s3r[1].Num != 5 {
		t.Error("s3r[1].Num is not 5")
	}

	s4, found := oc.get("structuration")
	if !found {
		t.Error("structuration was not found")
	}
	s4r := s4.(*TestStruct)
	if len(s4r.Children) != 2 {
		t.Error("Length of s4r.Children is not 2")
	}
	if s4r.Children[0].Num != 6174 {
		t.Error("s4r.Children[0].Num is not 6174")
	}
	if s4r.Children[1].Num != 4716 {
		t.Error("s4r.Children[1].Num is not 4716")
	}
}

func TestFileSerialization(t *testing.T) {
	tc := NewCache()
	_ = tc.Add("a", "a", DefaultExpire)
	_ = tc.Add("b", "b", DefaultExpire)
	f, err := ioutil.TempFile("", "go-cache-cache.dat")
	if err != nil {
		t.Fatal("Couldn't create cache file:", err)
	}
	path := f.Name()
	_ = f.Close()
	_ = tc.SaveFile(path)

	oc := NewCache()
	_ = oc.Add("a", "aa", 0) // this should not be overwritten
	err = oc.LoadFile(path)
	if err != nil {
		t.Error(err)
	}
	a, found := oc.Get("a")
	if !found {
		t.Error("a was not found")
	}
	aStr := a.(string)
	if aStr != "aa" {
		if aStr == "a" {
			t.Error("a was overwritten")
		} else {
			t.Error("a is not aa")
		}
	}
	b, found := oc.Get("b")
	if !found {
		t.Error("b was not found")
	}
	if b.(string) != "b" {
		t.Error("b is not b")
	}
}

func TestSerializeUnserializable(t *testing.T) {
	tc := NewCache()
	ch := make(chan bool, 1)
	ch <- true
	tc.Set("chan", ch, DefaultExpire)
	fp := &bytes.Buffer{}
	err := tc.Save(fp) // this should fail gracefully
	if err != nil && err.Error() != "gob NewTypeObject can't handle type: chan bool" {
		t.Error("Error from Save was not gob NewTypeObject can't handle type chan bool:", err)
	}
}

func BenchmarkCacheGetExpiring(b *testing.B) {
	benchmarkCacheGet(b)
}

func BenchmarkCacheGetNotExpiring(b *testing.B) {
	benchmarkCacheGet(b)
}

func benchmarkCacheGet(b *testing.B) {
	b.StopTimer()
	tc := NewCache()
	tc.Set("foo", "bar", DefaultExpire)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Get("foo")
	}
}

func BenchmarkRWMutexMapGet(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m["foo"]
		mu.RUnlock()
	}
}

func BenchmarkRWMutexInterfaceMapGetStruct(b *testing.B) {
	b.StopTimer()
	s := struct{ name string }{name: "foo"}
	m := map[interface{}]string{
		s: "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m[s]
		mu.RUnlock()
	}
}

func BenchmarkRWMutexInterfaceMapGetString(b *testing.B) {
	b.StopTimer()
	m := map[interface{}]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.RLock()
		_, _ = m["foo"]
		mu.RUnlock()
	}
}

func BenchmarkCacheGetConcurrentExpiring(b *testing.B) {
	benchmarkCacheGetConcurrent(b)
}

func BenchmarkCacheGetConcurrentNotExpiring(b *testing.B) {
	benchmarkCacheGetConcurrent(b)
}

func benchmarkCacheGetConcurrent(b *testing.B) {
	b.StopTimer()
	tc := NewCache()
	tc.Set("foo", "bar", DefaultExpire)
	wg := new(sync.WaitGroup)
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				tc.Get("foo")
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkRWMutexMapGetConcurrent(b *testing.B) {
	b.StopTimer()
	m := map[string]string{
		"foo": "bar",
	}
	mu := sync.RWMutex{}
	wg := new(sync.WaitGroup)
	workers := runtime.NumCPU()
	each := b.N / workers
	wg.Add(workers)
	b.StartTimer()
	for i := 0; i < workers; i++ {
		go func() {
			for j := 0; j < each; j++ {
				mu.RLock()
				_, _ = m["foo"]
				mu.RUnlock()
			}
			wg.Done()
		}()
	}
	wg.Wait()
}

func BenchmarkCacheGetManyConcurrentExpiring(b *testing.B) {
	benchmarkCacheGetManyConcurrent(b)
}

func BenchmarkCacheGetManyConcurrentNotExpiring(b *testing.B) {
	benchmarkCacheGetManyConcurrent(b)
}

func benchmarkCacheGetManyConcurrent(b *testing.B) {
	// This is the same as BenchmarkCacheGetConcurrent, but its result
	// can be compared against BenchmarkShardedCacheGetManyConcurrent
	// in sharded_test.go.
	b.StopTimer()
	n := 10000
	tc := NewCache()
	keys := make([]string, n)
	for i := 0; i < n; i++ {
		k := "foo" + strconv.Itoa(i)
		keys[i] = k
		tc.Set(k, "bar", DefaultExpire)
	}
	each := b.N / n
	wg := new(sync.WaitGroup)
	wg.Add(n)
	for _, v := range keys {
		go func(k string) {
			for j := 0; j < each; j++ {
				tc.Get(k)
			}
			wg.Done()
		}(v)
	}
	b.StartTimer()
	wg.Wait()
}

func BenchmarkCacheSetExpiring(b *testing.B) {
	benchmarkCacheSet(b)
}

func BenchmarkCacheSetNotExpiring(b *testing.B) {
	benchmarkCacheSet(b)
}

func benchmarkCacheSet(b *testing.B) {
	b.StopTimer()
	tc := NewCache()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Set("foo", "bar", DefaultExpire)
	}
}

func BenchmarkRWMutexMapSet(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		mu.Unlock()
	}
}

func BenchmarkCacheSetDelete(b *testing.B) {
	b.StopTimer()
	tc := NewCache(SetCapture(nil))
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Set("foo", "bar", DefaultExpire)
		tc.Delete("foo")
	}
}

func BenchmarkRWMutexMapSetDelete(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		mu.Unlock()
		mu.Lock()
		delete(m, "foo")
		mu.Unlock()
	}
}

func BenchmarkCacheSetDeleteSingleLock(b *testing.B) {
	b.StopTimer()
	tc := NewCache()
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.Lock()
		tc.set("foo", "bar", DefaultExpire)
		tc.delete("foo")
		tc.Unlock()
	}
}

func BenchmarkRWMutexMapSetDeleteSingleLock(b *testing.B) {
	b.StopTimer()
	m := map[string]string{}
	mu := sync.RWMutex{}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		mu.Lock()
		m["foo"] = "bar"
		delete(m, "foo")
		mu.Unlock()
	}
}

func BenchmarkIncrementInt(b *testing.B) {
	b.StopTimer()
	tc := NewCache()
	tc.Set("foo", 0, DefaultExpire)
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		_, _ = tc.IncrementInt("foo", 1)
	}
}

func BenchmarkDeleteExpiredLoop(b *testing.B) {
	b.StopTimer()
	tc := NewCache(SetDefaultExpire(5*time.Minute), SetCapture(nil))
	for i := 0; i < 100000; i++ {
		tc.set(strconv.Itoa(i), "bar", DefaultExpire)
	}
	b.StartTimer()
	for i := 0; i < b.N; i++ {
		tc.DeleteExpire()
	}
}

func TestGetWithExpiration(t *testing.T) {
	tc := NewCache()

	a, expiration, found := tc.GetWithExpire("a")
	if found || a != nil || !expiration.IsZero() {
		t.Error("Getting A found value that shouldn't exist:", a)
	}

	b, expiration, found := tc.GetWithExpire("b")
	if found || b != nil || !expiration.IsZero() {
		t.Error("Getting B found value that shouldn't exist:", b)
	}

	c, expiration, found := tc.GetWithExpire("c")
	if found || c != nil || !expiration.IsZero() {
		t.Error("Getting C found value that shouldn't exist:", c)
	}

	tc.Set("a", 1, DefaultExpire)
	tc.Set("b", "b", DefaultExpire)
	tc.Set("c", 3.5, DefaultExpire)
	tc.Set("d", 1, NoExpire)
	tc.Set("e", 1, 50*time.Millisecond)

	x, expiration, found := tc.GetWithExpire("a")
	if !found {
		t.Error("a was not found while getting a2")
	}
	if x == nil {
		t.Error("x for a is nil")
	} else if a2 := x.(int); a2+2 != 3 {
		t.Error("a2 (which should be 1) plus 2 does not equal 3; value:", a2)
	}
	if !expiration.IsZero() {
		t.Error("expiration for a is not a zeroed time")
	}

	x, expiration, found = tc.GetWithExpire("b")
	if !found {
		t.Error("b was not found while getting b2")
	}
	if x == nil {
		t.Error("x for b is nil")
	} else if b2 := x.(string); b2+"B" != "bB" {
		t.Error("b2 (which should be b) plus B does not equal bB; value:", b2)
	}
	if !expiration.IsZero() {
		t.Error("expiration for b is not a zeroed time")
	}

	x, expiration, found = tc.GetWithExpire("c")
	if !found {
		t.Error("c was not found while getting c2")
	}
	if x == nil {
		t.Error("x for c is nil")
	} else if c2 := x.(float64); c2+1.2 != 4.7 {
		t.Error("c2 (which should be 3.5) plus 1.2 does not equal 4.7; value:", c2)
	}
	if !expiration.IsZero() {
		t.Error("expiration for c is not a zeroed time")
	}

	x, expiration, found = tc.GetWithExpire("d")
	if !found {
		t.Error("d was not found while getting d2")
	}
	if x == nil {
		t.Error("x for d is nil")
	} else if d2 := x.(int); d2+2 != 3 {
		t.Error("d (which should be 1) plus 2 does not equal 3; value:", d2)
	}
	if !expiration.IsZero() {
		t.Error("expiration for d is not a zeroed time")
	}

	x, expiration, found = tc.GetWithExpire("e")
	if !found {
		t.Error("e was not found while getting e2")
	}
	if x == nil {
		t.Error("x for e is nil")
	} else if e2 := x.(int); e2+2 != 3 {
		t.Error("e (which should be 1) plus 2 does not equal 3; value:", e2)
	}
	if expiration.UnixNano() != tc.member["e"].Expire {
		t.Error("expiration for e is not the correct time")
	}
	if expiration.UnixNano() < time.Now().UnixNano() {
		t.Error("expiration for e is in the past")
	}
}
