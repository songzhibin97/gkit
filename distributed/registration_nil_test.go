package distributed

import (
	"errors"
	"testing"

	"github.com/songzhibin97/gkit/distributed/task"
)

func TestRegistrationRejectsNilWithoutWriting(t *testing.T) {
	var nilFunc func() error
	for _, test := range []struct {
		name string
		fn   interface{}
	}{
		{name: "typed_nil", fn: nilFunc},
		{name: "nil", fn: nil},
	} {
		for _, api := range []string{"RegisteredTask", "RegisteredTasks"} {
			t.Run(api+"/"+test.name, func(t *testing.T) {
				server := initServer(t)
				originalCalls := 0
				originalError := errors.New("synthetic existing task result")
				if err := server.RegisteredTask("existing", func() error { originalCalls++; return originalError }); err != nil {
					t.Fatal(err)
				}
				for _, key := range []string{"invalid", "existing"} {
					var err error
					if api == "RegisteredTask" {
						err = server.RegisteredTask(key, test.fn)
					} else {
						err = server.RegisteredTasks(map[string]interface{}{key: test.fn})
					}
					if !errors.Is(err, task.ErrTaskMustFunc) {
						t.Errorf("registration error = %v, want %v", err, task.ErrTaskMustFunc)
					}
				}
				if _, found := server.GetRegisteredTask("invalid"); found || server.IsRegisteredTask("invalid") || server.GetController().IsRegisterTask("invalid") {
					t.Error("nil function was registered in the server or controller")
				}
				original, found := server.GetRegisteredTask("existing")
				if !found {
					t.Fatal("existing task was removed")
				}
				fn, ok := original.(func() error)
				if !ok || fn == nil {
					t.Fatal("existing task was replaced by a non-callable value")
				}
				if err := fn(); err != originalError || originalCalls != 1 {
					t.Fatalf("existing task changed: err=%v calls=%d", err, originalCalls)
				}
			})
		}
	}
}
