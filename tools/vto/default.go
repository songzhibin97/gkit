package vto

import (
	"reflect"

	"github.com/songzhibin97/gkit/tools"
)

func CompletionDefault(dst interface{}) error {
	dstT := reflect.TypeOf(dst)
	if dstT.Kind() != reflect.Ptr {
		return tools.ErrorMustPtr
	}

	dstT = dstT.Elem()
	if dstT.Kind() != reflect.Struct {
		return tools.ErrorMustStructPtr
	}

	dstV := reflect.ValueOf(dst).Elem()
	for i := 0; i < dstT.NumField(); i++ {
		field := dstT.Field(i)
		if !field.IsExported() {
			continue
		}
		d := dstV.Field(i)

		if d.Kind() == reflect.Struct {
			err := CompletionDefault(d.Addr().Interface())
			if err != nil {
				return err
			}
			continue
		}

		defaultTag := field.Tag.Get("default")
		if defaultTag == "" {
			continue
		}

		if d.IsZero() {
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
