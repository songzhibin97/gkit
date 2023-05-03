package vto

import (
	"reflect"

	"github.com/songzhibin97/gkit/tools"

	"github.com/songzhibin97/gkit/tools/bind"
)

type BindModel int

const (
	FieldBind BindModel = 1 << iota
	TagBind
	DefaultValueBind // 默认值绑定,在最后的情况下,如果还未绑定到相关值则设置为默认值
	OverlayBind      // 设置多个条件的情况下可以覆盖,根据代码语义,应该是会以tag优先,不设置以field为优先
)

type ModelParameters struct {
	Model BindModel `json:"model"` // 绑定参数 默认值为 field bind
	Tag   string    `json:"tag"`   // tag bind 指定tag,default 为 json
}

// VoToDo 试图对象与domino对象转换,只能转相同字段且类型相同的
// dst: 目标
// src: 源位置
// 支持简单的 default模式 在基础类型增加default可以指定默认值
func VoToDo(dst interface{}, src interface{}) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT.Kind() != srcT.Kind() {
		return tools.ErrorNoEquals
	}
	if dstT.Kind() != reflect.Ptr {
		return tools.ErrorMustPtr
	}
	dstT, srcT = dstT.Elem(), srcT.Elem()
	if dstT.Kind() != reflect.Struct || srcT.Kind() != reflect.Struct {
		return tools.ErrorMustStructPtr
	}
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()
	for i := 0; i < dstT.NumField(); i++ {
		field := dstT.Field(i)
		if !field.IsExported() {
			continue
		}
		defaultTag := field.Tag.Get("default")
		if _, ok := srcT.FieldByName(field.Name); !ok {
			continue
		}
		d := dstV.Field(i)
		s := srcV.FieldByName(field.Name)
		for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
			s = s.Elem()
		}

		if d.Kind() != s.Kind() {
			continue
		}

		if d.Kind() != s.Kind() && d.Kind() == reflect.Struct {
			err := VoToDo(d.Addr().Interface(), s.Addr().Interface())
			if err != nil {
				return err
			}
			continue
		}

		if !s.IsZero() {
			if d.Type() == s.Type() {
				d.Set(s)
			} else {
				d.Set(reflect.ValueOf(s.Interface()).Convert(d.Type()))
			}
		}

		// 如果源位置的内容为空,并且默认值不为0
		if d.IsZero() && len(defaultTag) > 0 {
			if d.Kind() == reflect.Ptr {
				ss := reflect.New(d.Type().Elem())
				err := bindDefault(ss.Elem(), defaultTag, field)
				if err != nil {
					return err
				}
				d.Set(ss)
			} else {
				err := bindDefault(d, defaultTag, field)
				if err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// VoToDoPlus View对象与domino对象转换,根据不同模式进行转换
// dst: 目标
// src: 源位置
// ModelParameters: 模式匹配
func VoToDoPlus(dst interface{}, src interface{}, model ModelParameters) error {
	dstT, srcT := reflect.TypeOf(dst), reflect.TypeOf(src)
	if dstT.Kind() != srcT.Kind() {
		return tools.ErrorNoEquals
	}
	if dstT.Kind() != reflect.Ptr {
		return tools.ErrorMustPtr
	}
	dstT, srcT = dstT.Elem(), srcT.Elem()
	if dstT.Kind() != reflect.Struct || srcT.Kind() != reflect.Struct {
		return tools.ErrorMustStructPtr
	}
	dstV, srcV := reflect.ValueOf(dst).Elem(), reflect.ValueOf(src).Elem()

	if model.Model&TagBind == TagBind && len(model.Tag) == 0 {
		model.Tag = "json"
	}

	if model.Model&FieldBind == FieldBind {
		for i := 0; i < dstT.NumField(); i++ {
			field := dstT.Field(i)
			if !field.IsExported() {
				continue
			}
			d := dstV.Field(i)
			if _, ok := srcT.FieldByName(field.Name); !ok {
				continue
			}

			s := srcV.FieldByName(field.Name)
			for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
				s = s.Elem()
			}
			if d.Kind() != s.Kind() {
				continue
			}

			if d.Kind() != s.Kind() && d.Kind() == reflect.Struct {
				err := VoToDoPlus(d.Addr().Interface(), s.Addr().Interface(), model)
				if err != nil {
					return err
				}
				continue
			}

			if !s.IsZero() {
				if d.Type() == s.Type() {
					d.Set(s)
				} else {
					d.Set(reflect.ValueOf(s.Interface()).Convert(d.Type()))
				}
			}
		}
	}

	if model.Model&TagBind == TagBind {
		for i := 0; i < dstT.NumField(); i++ {
			field := dstT.Field(i)
			if !field.IsExported() {
				continue
			}
			d := dstV.Field(i)

			if !d.IsZero() && !(model.Model&OverlayBind == OverlayBind) {
				continue
			}

			currentFieldTag := field.Tag.Get(model.Tag)

			// 这里没办法,就需要循环 on^2
			for j := 0; j < srcT.NumField(); j++ {
				srcField := srcT.Field(j)
				if currentFieldTag == srcField.Tag.Get(model.Tag) {

					s := srcV.FieldByName(srcField.Name)
					for s.Kind() == reflect.Ptr && d.Kind() != s.Kind() {
						s = s.Elem()
					}
					if d.Kind() != s.Kind() {
						continue
					}

					if d.Type() != s.Type() && d.Kind() == reflect.Struct {
						err := VoToDoPlus(d.Addr().Interface(), s.Addr().Interface(), model)
						if err != nil {
							return err
						}
						continue
					}

					if !s.IsZero() {
						d.Set(s)
					}
				}
			}
		}
	}

	if model.Model&DefaultValueBind == DefaultValueBind {
		for i := 0; i < dstT.NumField(); i++ {
			field := dstT.Field(i)
			if !field.IsExported() {
				continue
			}
			defaultTag := field.Tag.Get("default")
			if len(defaultTag) == 0 || defaultTag == "-" {
				continue
			}
			d := dstV.Field(i)
			// 如果源位置的内容为空,并且默认值不为0
			if d.IsZero() && len(defaultTag) > 0 {
				if d.Kind() == reflect.Ptr {
					s := reflect.New(d.Type().Elem())
					err := bindDefault(s.Elem(), defaultTag, field)
					if err != nil {
						return err
					}
					d.Set(s)
				} else {
					err := bindDefault(d, defaultTag, field)
					if err != nil {
						return err
					}
				}
			}
		}
	}

	return nil
}

func bindDefault(value reflect.Value, val string, field reflect.StructField) error {
	return bind.SetWithProperType(val, value, field)
}
