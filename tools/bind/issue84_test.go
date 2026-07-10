package bind

import (
	"context"
	"io"
	"net/http/httptest"
	"net/url"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"

	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/reflect/protoreflect"
	"google.golang.org/protobuf/types/known/wrapperspb"
)

type issue84NilMapMessage map[string]string

func (issue84NilMapMessage) ProtoReflect() protoreflect.Message {
	panic("typed-nil protobuf target must be rejected before ProtoReflect")
}

func TestProtobufBindingRejectsNonMessage(t *testing.T) {
	var target struct {
		Value string
	}
	err := ProtoBuf.BindBody(nil, &target)
	if err == nil {
		t.Fatal("BindBody() error = nil, want a protobuf target type error")
	}
	if !strings.Contains(err.Error(), "does not implement proto.Message") {
		t.Fatalf("BindBody() error = %q, want explicit protobuf target type error", err)
	}
}

func TestProtobufBindingDecodesMessage(t *testing.T) {
	want := wrapperspb.String("decoded")
	body, err := proto.Marshal(want)
	if err != nil {
		t.Fatalf("proto.Marshal() error = %v", err)
	}

	got := new(wrapperspb.StringValue)
	if err := ProtoBuf.BindBody(body, got); err != nil {
		t.Fatalf("BindBody() error = %v", err)
	}
	if !proto.Equal(got, want) {
		t.Fatalf("BindBody() = %v, want %v", got, want)
	}
}

func TestProtobufBindingRejectsTypedNilMessage(t *testing.T) {
	tests := []struct {
		name    string
		message proto.Message
	}{
		{name: "pointer", message: (*wrapperspb.StringValue)(nil)},
		{name: "map", message: issue84NilMapMessage(nil)},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ProtoBuf.BindBody(nil, tt.message)
			if err == nil {
				t.Fatal("BindBody() error = nil, want a typed-nil target error")
			}
			if !strings.Contains(err.Error(), "nil proto.Message") {
				t.Fatalf("BindBody() error = %q, want explicit typed-nil target error", err)
			}
		})
	}
}

type issue84RecursiveForm struct {
	Value string                `json:"value" form:"value"`
	Next  *issue84RecursiveForm `form:"next"`
}

func TestFormBindingEmptySelfReferenceReturns(t *testing.T) {
	const helperEnv = "GKIT_ISSUE84_BIND_SELF_REFERENCE_HELPER"
	if os.Getenv(helperEnv) == "1" {
		queryRequest := httptest.NewRequest("GET", "/", nil)
		var queryTarget issue84RecursiveForm
		if err := Query.Bind(queryRequest, &queryTarget); err != nil {
			t.Fatalf("Query.Bind() error = %v", err)
		}
		if queryTarget.Next != nil {
			t.Fatalf("Query.Bind() allocated Next without input: %#v", queryTarget.Next)
		}

		formRequest := httptest.NewRequest("POST", "/", strings.NewReader(""))
		formRequest.Header.Set("Content-Type", MIMEPOSTForm)
		var formTarget issue84RecursiveForm
		if err := Form.Bind(formRequest, &formTarget); err != nil {
			t.Fatalf("Form.Bind() error = %v", err)
		}
		if formTarget.Next != nil {
			t.Fatalf("Form.Bind() allocated Next without input: %#v", formTarget.Next)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFormBindingEmptySelfReferenceReturns$", "-test.count=1")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("self-referential form binding did not return: %v", ctx.Err())
		}
		t.Fatalf("self-referential form binding subprocess failed: %v", err)
	}
}

func TestFormBindingExistingSelfReferenceReturns(t *testing.T) {
	const helperEnv = "GKIT_ISSUE84_BIND_EXISTING_SELF_REFERENCE_HELPER"
	if os.Getenv(helperEnv) == "1" {
		queryRequest := httptest.NewRequest("GET", "/", nil)
		var queryTarget issue84RecursiveForm
		queryTarget.Next = &queryTarget
		if err := Query.Bind(queryRequest, &queryTarget); err != nil {
			t.Fatalf("Query.Bind() error = %v", err)
		}
		if queryTarget.Next != &queryTarget {
			t.Fatalf("Query.Bind() changed the existing cycle: %#v", queryTarget.Next)
		}

		formRequest := httptest.NewRequest("POST", "/", strings.NewReader(""))
		formRequest.Header.Set("Content-Type", MIMEPOSTForm)
		var formTarget issue84RecursiveForm
		formTarget.Next = &formTarget
		if err := Form.Bind(formRequest, &formTarget); err != nil {
			t.Fatalf("Form.Bind() error = %v", err)
		}
		if formTarget.Next != &formTarget {
			t.Fatalf("Form.Bind() changed the existing cycle: %#v", formTarget.Next)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, os.Args[0], "-test.run=^TestFormBindingExistingSelfReferenceReturns$", "-test.count=1")
	cmd.Env = append(os.Environ(), helperEnv+"=1")
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Run(); err != nil {
		if ctx.Err() != nil {
			t.Fatalf("existing self-referential form binding did not return: %v", ctx.Err())
		}
		t.Fatalf("existing self-referential form binding subprocess failed: %v", err)
	}
}

func TestFormBindingSelfReferenceAllocatesForInput(t *testing.T) {
	values := make(url.Values)
	values.Set("next", `{"value":"child"}`)
	queryRequest := httptest.NewRequest("GET", "/?"+values.Encode(), nil)
	var queryTarget issue84RecursiveForm
	if err := Query.Bind(queryRequest, &queryTarget); err != nil {
		t.Fatalf("Query.Bind() error = %v", err)
	}
	assertOneChild := func(name string, got issue84RecursiveForm) {
		t.Helper()
		if got.Next == nil || got.Next.Value != "child" {
			t.Fatalf("%s Next = %#v, want one allocated child", name, got.Next)
		}
		if got.Next.Next != nil {
			t.Fatalf("%s allocated an unrequested grandchild: %#v", name, got.Next.Next)
		}
	}
	assertOneChild("Query.Bind()", queryTarget)

	formRequest := httptest.NewRequest("POST", "/", strings.NewReader(values.Encode()))
	formRequest.Header.Set("Content-Type", MIMEPOSTForm)
	var formTarget issue84RecursiveForm
	if err := Form.Bind(formRequest, &formTarget); err != nil {
		t.Fatalf("Form.Bind() error = %v", err)
	}
	assertOneChild("Form.Bind()", formTarget)
}

func TestFormBindingTraversesExistingAcyclicChain(t *testing.T) {
	child := &issue84RecursiveForm{Value: "before"}
	target := issue84RecursiveForm{Value: "before", Next: child}
	values := make(url.Values)
	values.Set("next", `{"value":"after"}`)
	request := httptest.NewRequest("GET", "/?"+values.Encode(), nil)

	if err := Query.Bind(request, &target); err != nil {
		t.Fatalf("Query.Bind() error = %v", err)
	}
	if target.Value != "before" || target.Next != child || child.Value != "after" {
		t.Fatalf("Query.Bind() target = %#v child = %#v, want existing chain updated in place", target, child)
	}
	if child.Next != nil {
		t.Fatalf("Query.Bind() allocated an unrequested grandchild: %#v", child.Next)
	}
}
