package errors

import (
	"testing"

	"google.golang.org/genproto/googleapis/rpc/errdetails"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

func TestFromErrorPreservesStandardGRPCStatus(t *testing.T) {
	got := FromError(status.Error(codes.NotFound, "missing"))
	if got.Code != 404 {
		t.Fatalf("FromError code = %d, want 404", got.Code)
	}
	if got.Reason != "" {
		t.Fatalf("FromError reason = %q, want empty", got.Reason)
	}
	if got.Message != "missing" {
		t.Fatalf("FromError message = %q, want missing", got.Message)
	}
}

func TestFromErrorKeepsErrorInfoDetails(t *testing.T) {
	grpcStatus, err := status.New(codes.PermissionDenied, "denied").WithDetails(&errdetails.ErrorInfo{
		Reason:   "POLICY",
		Metadata: map[string]string{"scope": "write"},
	})
	if err != nil {
		t.Fatal(err)
	}

	got := FromError(grpcStatus.Err())
	if got.Code != 403 || got.Reason != "POLICY" || got.Message != "denied" {
		t.Fatalf("FromError detail = %#v, want code=403 reason=POLICY message=denied", got)
	}
	if got.Metadata["scope"] != "write" {
		t.Fatalf("FromError metadata = %#v, want scope=write", got.Metadata)
	}
}
