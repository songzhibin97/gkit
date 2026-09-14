package backend_db

import (
	"database/sql"
	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
	"os"
	"testing"
)

func TestIssue167ResultPrecision(t *testing.T) {
	for _, tt := range []struct{ name, env, driver, kind string }{{"mysql", "GKIT_MYSQL_DSN", "mysql", "mysql"}, {"postgres", "GKIT_POSTGRES_DSN", "pgx", "pgsql"}} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip("#167 live precision requires " + tt.env + "; owned fixture required before acceptance")
			}
			db, err := sql.Open(tt.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			b, err := NewBackendSQLDBE(db, -1, tt.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			chordtest.RunResultPrecision(t, b.(*BackendSQLDB))
		})
	}
}

func TestIssue167ResultConsumers(t *testing.T) {
	for _, tt := range []struct{ name, env, driver, kind string }{{"mysql", "GKIT_MYSQL_DSN", "mysql", "mysql"}, {"postgres", "GKIT_POSTGRES_DSN", "pgx", "pgsql"}} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip("#167 live precision requires " + tt.env + "; owned fixture required before acceptance")
			}
			db, err := sql.Open(tt.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			b, err := NewBackendSQLDBE(db, -1, tt.kind, nil)
			if err != nil {
				t.Fatal(err)
			}
			chordtest.RunResultConsumers(t, b.(*BackendSQLDB))
		})
	}
}
