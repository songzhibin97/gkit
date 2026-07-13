package bind

import (
	"bytes"
	"errors"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"strings"
	"testing"
)

func TestFormBindingsAllocateNilMapPointers(t *testing.T) {
	values := url.Values{"name": {"first", "last"}}
	binders := []struct {
		name string
		bind func(interface{}) error
	}{
		{
			name: "uri",
			bind: func(target interface{}) error { return Uri.BindUri(values, target) },
		},
		{
			name: "form",
			bind: func(target interface{}) error {
				request := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
				request.Header.Set("Content-Type", MIMEPOSTForm)
				return Form.Bind(request, target)
			},
		},
	}

	for _, binder := range binders {
		t.Run(binder.name+"/map-string", func(t *testing.T) {
			var target map[string]string
			if err := binder.bind(&target); err != nil {
				t.Fatalf("bind nil map pointer: %v", err)
			}
			if target == nil || target["name"] != "last" {
				t.Fatalf("bound map = %#v, want allocated map with last value", target)
			}
		})

		t.Run(binder.name+"/map-string-slice", func(t *testing.T) {
			var target map[string][]string
			if err := binder.bind(&target); err != nil {
				t.Fatalf("bind nil map-slice pointer: %v", err)
			}
			if target == nil || !reflect.DeepEqual(target["name"], []string{"first", "last"}) {
				t.Fatalf("bound map = %#v, want allocated map with all values", target)
			}
		})
	}
}

func TestFormBindingsRejectInvalidOrUnsettableTargets(t *testing.T) {
	values := url.Values{"name": {"value"}}
	type targetStruct struct {
		Name string `form:"name" uri:"name"`
	}
	var typedNil *targetStruct
	var nilMap map[string]string
	scalar := 1
	nonStringKeyMap := map[int]string{}

	binders := []struct {
		name string
		bind func(interface{}) error
	}{
		{name: "uri", bind: func(target interface{}) error { return Uri.BindUri(values, target) }},
		{
			name: "form",
			bind: func(target interface{}) error {
				request := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
				request.Header.Set("Content-Type", MIMEPOSTForm)
				return Form.Bind(request, target)
			},
		},
	}
	invalid := []struct {
		name   string
		target interface{}
	}{
		{name: "untyped-nil", target: nil},
		{name: "typed-nil-pointer", target: typedNil},
		{name: "non-pointer-struct", target: targetStruct{}},
		{name: "unsettable-nil-map", target: nilMap},
		{name: "pointer-to-scalar", target: &scalar},
		{name: "non-string-map-key", target: &nonStringKeyMap},
	}

	for _, binder := range binders {
		for _, invalidTarget := range invalid {
			t.Run(binder.name+"/"+invalidTarget.name, func(t *testing.T) {
				if err := binder.bind(invalidTarget.target); !errors.Is(err, errUnknownType) {
					t.Fatalf("bind error = %v, want %v", err, errUnknownType)
				}
			})
		}

		t.Run(binder.name+"/unsupported-map", func(t *testing.T) {
			var target map[string]int
			if err := binder.bind(&target); err == nil || !strings.Contains(err.Error(), "cannot convert to map of strings") {
				t.Fatalf("bind error = %v, want existing map conversion error", err)
			}
		})
	}
}

func TestMappingByPtrBindingsRejectNilTargets(t *testing.T) {
	type targetStruct struct {
		Name string `form:"name" header:"name"`
	}
	var typedNil *targetStruct

	binders := []struct {
		name string
		bind func(interface{}) error
	}{
		{
			name: "header",
			bind: func(target interface{}) error {
				request := httptest.NewRequest(http.MethodGet, "/", nil)
				request.Header.Set("name", "value")
				return Header.Bind(request, target)
			},
		},
		{
			name: "form-multipart",
			bind: func(target interface{}) error {
				body := &bytes.Buffer{}
				writer := multipart.NewWriter(body)
				if err := writer.WriteField("name", "value"); err != nil {
					t.Fatalf("write multipart field: %v", err)
				}
				if err := writer.Close(); err != nil {
					t.Fatalf("close multipart writer: %v", err)
				}
				request := httptest.NewRequest(http.MethodPost, "/", body)
				request.Header.Set("Content-Type", writer.FormDataContentType())
				return FormMultipart.Bind(request, target)
			},
		},
	}
	invalid := []struct {
		name   string
		target interface{}
	}{
		{name: "untyped-nil", target: nil},
		{name: "typed-nil-pointer", target: typedNil},
	}

	for _, binder := range binders {
		for _, invalidTarget := range invalid {
			t.Run(binder.name+"/"+invalidTarget.name, func(t *testing.T) {
				if err := binder.bind(invalidTarget.target); !errors.Is(err, errUnknownType) {
					t.Fatalf("bind error = %v, want %v", err, errUnknownType)
				}
			})
		}
	}
}
