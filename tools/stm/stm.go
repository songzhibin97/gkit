package stm

import (
	"reflect"
	"unsafe"
)

// struct => map

func StructToMap(val interface{}, tag string) map[string]interface{} {
	return structToMap(val, tag, false)
}

func StructToMapExtraExport(val interface{}, tag string) map[string]interface{} {
	return structToMap(val, tag, true)
}

func structToMap(val interface{}, tag string, extraExport bool) map[string]interface{} {
	t, v := reflect.TypeOf(val), reflect.ValueOf(val)
	if v.Kind() == reflect.Ptr {
		return structToMap(v.Elem().Interface(), tag, extraExport)
	}
	if v.Kind() != reflect.Struct {
		return nil
	}
	mp := make(map[string]interface{})

	for i := 0; i < v.NumField(); i++ {
		vField := v.Field(i)
		name := t.Field(i).Name
		if tag != "" {
			ts := t.Field(i).Tag.Get(tag)
			if ts == "-" {
				continue
			}
			if ts != "" {
				name = ts
			}
		}
		if !vField.IsValid() {
			continue
		}
		st := (vField.Kind() == reflect.Struct) || (vField.Kind() == reflect.Ptr && vField.Elem().Kind() == reflect.Struct)
		if vField.CanInterface() {
			iv := vField.Interface()
			if st {
				iv = structToMap(iv, tag, extraExport)
			}
			mp[name] = iv
		} else if extraExport {
			// 未导出
			cp := reflect.New(v.Type()).Elem()
			cp.Set(v)
			value := cp.Field(i)
			iv := reflect.NewAt(value.Type(), unsafe.Pointer(value.UnsafeAddr())).Elem().Interface()
			if st {
				iv = structToMap(iv, tag, extraExport)
			}
			mp[name] = iv
		}
	}
	return mp
}
