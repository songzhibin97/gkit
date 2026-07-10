package task

import "testing"

func TestNewSignatureDefaultRetryIntervalIsSeconds(t *testing.T) {
	signature := NewSignature("task-id", "task-name")
	if signature.RetryInterval != 60 {
		t.Fatalf("RetryInterval = %d, want 60 seconds", signature.RetryInterval)
	}
}

func TestNewSignatureRouterIsUnset(t *testing.T) {
	signature := NewSignature("task-id", "task-name")
	if signature.Router != "" {
		t.Fatalf("Router = %q, want empty unset sentinel", signature.Router)
	}
}
