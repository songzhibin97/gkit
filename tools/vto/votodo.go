package vto

import (
	"errors"
	"fmt"
	"github.com/songzhibin97/gkit/tools/bind"
	"reflect"
)

// VoToDo 试图对象与domino对象转换,只能转相同字段且类型相同的
// dst: 目标
// src: 源位置
// 支持简单的 default模式 在基础类型增加default可以指定默认值
func VoToDo(dst interface{}, src interface{}) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT.Kind() != reflect.Ptr || srcT.Kind() != reflect.Ptr {
		return errors.New("dst 或 src 必须是指针类型")
	}
	dstT, srcT = dstT.Elem(), srcT.Elem()
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	for i := 0; i < dstT.NumField(); i++ {
		field := dstT.Field(i)
		defaultTag := field.Tag.Get("default")
		if !field.IsExported() {
			continue
		}
		if _, ok := srcT.FieldByName(field.Name); ok {
			d := dstV.Field(i)
			s := srcV.FieldByName(field.Name)
			for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
				s = s.Elem()
			}
			if d.Kind() == s.Kind() {
				var bindTag = true
				if s.IsZero() && len(defaultTag) > 0 {
					if d.Kind() == reflect.Ptr {
						s = reflect.New(d.Type().Elem())
						err := bindDefault(s.Elem(), defaultTag, field)
						if err != nil {
							return err
						}
					} else {
						err := bindDefault(d, defaultTag, field)
						if err != nil {
							return err
						}
						bindTag = false
					}
				}
				if bindTag {
					d.Set(s)
				}
			}
		}
	}
	return nil
}

func bindDefault(value reflect.Value, df string, field reflect.StructField) error {
	vs := []string{df}
	switch value.Kind() {
	case reflect.Slice:
		return bind.SetSlice(vs, value, field)
	case reflect.Array:
		if len(vs) != value.Len() {
			return fmt.Errorf("%q is not valid value for %s", vs, value.Type().String())
		}
		return bind.SetArray(vs, value, field)
	default:
		var val string
		val = df
		if len(vs) > 0 {
			val = vs[0]
		}
		return bind.SetWithProperType(val, value, field)
	}
}