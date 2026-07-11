package backend_db

import (
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestPublicationAttemptCompensationLiveSQL(t *testing.T) {
	tests := []struct {
		name       string
		env        string
		driverName string
		dbType     string
	}{
		{name: "mysql", env: "GKIT_MYSQL_DSN", driverName: "mysql", dbType: "mysql"},
		{name: "postgres", env: "GKIT_POSTGRES_DSN", driverName: "pgx", dbType: "pgsql"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip(tt.env + " is not set")
			}
			sqlDB, err := sql.Open(tt.driverName, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = sqlDB.Close() })
			prefix := fmt.Sprintf("gkit_attempt_%d_", time.Now().UnixNano())
			constructed, err := NewBackendSQLDBE(sqlDB, -1, tt.dbType, &gorm.Config{
				NamingStrategy: schema.NamingStrategy{TablePrefix: prefix},
			})
			if err != nil {
				t.Fatal(err)
			}
			backend, ok := constructed.(*BackendSQLDB)
			if !ok {
				t.Fatalf("backend type = %T, want *BackendSQLDB", constructed)
			}
			t.Cleanup(func() {
				if err := backend.gClient.Migrator().DropTable(&task.GroupMeta{}, &task.Status{}); err != nil {
					t.Errorf("drop isolated integration tables: %v", err)
				}
			})

			signature := &task.Signature{ID: "live-sql-attempt", GroupID: "group", Name: "task"}
			if err := backend.SetStatePendingAttempt(signature, "attempt-a"); err != nil {
				t.Fatal(err)
			}
			if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "publish failed"); err != nil || !changed {
				t.Fatalf("matching compensation = (%t, %v), want true, nil", changed, err)
			}
			if err := backend.SetStatePendingAttempt(signature, "attempt-b"); err != nil {
				t.Fatal(err)
			}
			if changed, err := backend.FailPendingAttempt(signature, "attempt-a", "stale attempt"); err != nil || changed {
				t.Fatalf("stale compensation = (%t, %v), want false, nil", changed, err)
			}
			status, err := backend.GetStatus(signature.ID)
			if err != nil || status.Status != task.StatePending || status.Error != "" {
				t.Fatalf("final status = %#v, %v", status, err)
			}
		})
	}
}
