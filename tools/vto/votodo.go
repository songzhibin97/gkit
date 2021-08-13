package vto

import (
	"errors"
	"reflect"
)

// VoToDo 试图对象与domino对象转换,只能转相同字段且类型相同的
// dst: 目标
// src: 源位置
func VoToDo(dst interface{}, src interface{}) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT.Kind() != reflect.Ptr || srcT.Kind() != reflect.Ptr {
		return errors.New("dst 或 src 必须是指针类型")
	}
	dstT, srcT = dstT.Elem(), srcT.Elem()
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	for i := 0; i < dstT.NumField(); i++ {
		name := dstT.Field(i).Name
		if _, ok := srcT.FieldByName(name); ok {
			d := dstV.Field(i)
			s := srcV.FieldByName(name)
			if d.Kind() == s.Kind() {
				d.Set(s)
			}
			for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
				s = s.Elem()
			}
			if d.Kind() == s.Kind() {
				d.Set(s)
			}
		}
	}
	return nil
}
