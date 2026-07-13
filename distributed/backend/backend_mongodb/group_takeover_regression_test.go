package backend_mongodb

import (
	"errors"
	"strings"
	"testing"

	"github.com/songzhibin97/gkit/distributed/backend"
	"go.mongodb.org/mongo-driver/mongo/integration/mtest"
)

// TestGroupTakeOverDuplicateKeyError runs against the driver's mock deployment,
// so it needs no real MongoDB. It pins the cross-backend contract from PR #99:
// a duplicate group takeover must surface backend.ErrGroupAlreadyExists so
// callers such as durable chord replays can tolerate it with errors.Is.
func TestGroupTakeOverDuplicateKeyError(t *testing.T) {
	mt := mtest.New(t, mtest.NewOptions().ClientType(mtest.Mock))
	defer mt.Close()

	mt.Run("duplicate key maps to shared sentinel", func(mt *mtest.T) {
		// A negative retention keeps the constructor free of MongoDB I/O.
		b, err := NewBackendMongoDBE(mt.Client, -1)
		if err != nil {
			mt.Fatalf("NewBackendMongoDBE() error = %v", err)
		}
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    11000,
			Message: "E11000 duplicate key error collection: gkit.groups index: _id_ dup key",
		}))
		err = b.GroupTakeOver("group-1", "group", "task-1")
		if !errors.Is(err, backend.ErrGroupAlreadyExists) {
			mt.Fatalf("duplicate GroupTakeOver error = %v, want backend.ErrGroupAlreadyExists", err)
		}
		if want := `take over group "group-1"`; !strings.Contains(err.Error(), want) {
			mt.Fatalf("duplicate GroupTakeOver error = %v, want message containing %q", err, want)
		}
	})

	mt.Run("non-duplicate write error passes through", func(mt *mtest.T) {
		b, err := NewBackendMongoDBE(mt.Client, -1)
		if err != nil {
			mt.Fatalf("NewBackendMongoDBE() error = %v", err)
		}
		mt.AddMockResponses(mtest.CreateWriteErrorsResponse(mtest.WriteError{
			Index:   0,
			Code:    121, // DocumentValidationFailure
			Message: "Document failed validation",
		}))
		err = b.GroupTakeOver("group-1", "group", "task-1")
		if err == nil {
			mt.Fatal("GroupTakeOver error = nil, want the underlying write error")
		}
		if errors.Is(err, backend.ErrGroupAlreadyExists) {
			mt.Fatalf("GroupTakeOver error = %v, must not map non-duplicate errors to ErrGroupAlreadyExists", err)
		}
	})
}
