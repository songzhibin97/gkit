package backend_db

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/songzhibin97/gkit/distributed/backend/result"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"os"
	"testing"
	"time"
)

func TestIssue167SQLReadDeadline(t *testing.T) {
	for _, tt := range []struct{ name, env, driver, kind string }{{"mysql", "GKIT_MYSQL_DSN", "mysql", "mysql"}, {"postgres", "GKIT_POSTGRES_DSN", "pgx", "pgsql"}} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip("#167 live timeout requires " + tt.env + "; owned fixture required before acceptance")
			}
			db, err := sql.Open(tt.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			b, err := NewBackendSQLDBE(db, -1, tt.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			held, err := db.Conn(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := held.Close(); err != nil && !errors.Is(err, sql.ErrConnDone) {
					t.Error(err)
				}
			}()
			done := make(chan error, 1)
			go func() {
				_, err := result.NewAsyncResult(&task.Signature{ID: "pool-wait-deadline"}, b).GetWithTimeout(20*time.Millisecond, time.Hour)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Fatalf("pool read timeout: %v", err)
				}
			case <-time.After(time.Second):
				if err := held.Close(); err != nil {
					t.Error(err)
				}
				<-done
				t.Fatal("pool wait exceeded timeout")
			}
		})
	}
}

func TestIssue167SQLExpiryCleanupDeadline(t *testing.T) {
	for _, tt := range []struct{ name, env, driver, kind string }{{"mysql", "GKIT_MYSQL_DSN", "mysql", "mysql"}, {"postgres", "GKIT_POSTGRES_DSN", "pgx", "pgsql"}} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip("#167 live timeout requires " + tt.env + "; owned fixture required before acceptance")
			}
			db, err := sql.Open(tt.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			defer func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			}()
			value, err := NewBackendSQLDBE(db, -1, tt.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			b := value.(*BackendSQLDB)
			id := fmt.Sprintf("issue167-expired-context-%d", time.Now().UnixNano())
			if err := b.gClient.Create(&task.Status{TaskID: id, TTL: 1, CreateAt: time.Now().Add(-time.Hour)}).Error; err != nil {
				t.Fatal(err)
			}
			release := make(chan struct{})
			if err := b.gClient.Callback().Delete().Before("gorm:delete").Register("issue167-expiry-context", func(tx *gorm.DB) {
				select {
				case <-tx.Statement.Context.Done():
					tx.AddError(tx.Statement.Context.Err())
				case <-release:
				}
			}); err != nil {
				t.Fatal(err)
			}
			done := make(chan error, 1)
			go func() {
				_, err := result.NewAsyncResult(&task.Signature{ID: id}, b).GetWithTimeout(20*time.Millisecond, time.Hour)
				done <- err
			}()
			select {
			case err := <-done:
				if !errors.Is(err, context.DeadlineExceeded) {
					t.Errorf("expiry cleanup timeout: %v", err)
				}
			case <-time.After(time.Second):
				close(release)
				<-done
				t.Error("expiry cleanup exceeded timeout")
			}
			if err := b.gClient.Callback().Delete().Remove("issue167-expiry-context"); err != nil {
				t.Fatal(err)
			}
			// The canceled deletion must leave the expired row for a healthy retry.
			var count int64
			if err := b.gClient.Model(&task.Status{}).Where("id = ?", id).Count(&count).Error; err != nil {
				t.Fatal(err)
			}
			if count != 1 {
				t.Errorf("expired rows after canceled cleanup = %d", count)
			}
			if _, err := b.GetStatus(id); !errors.Is(err, gorm.ErrRecordNotFound) {
				t.Fatalf("healthy expiry retry: %v", err)
			}
		})
	}
}
