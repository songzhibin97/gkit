package backend_db

import (
	"database/sql"
	"os"
	"testing"

	"github.com/glebarez/sqlite"
	"github.com/songzhibin97/gkit/distributed/backend/chordtest"
	"gorm.io/gorm"
)

func TestDurableChordContract(t *testing.T) {
	db, err := gorm.Open(sqlite.Open("file::memory:?cache=shared"), &gorm.Config{})
	if err != nil {
		t.Fatal(err)
	}
	backend := &BackendSQLDB{gClient: db, resultExpire: -1}
	if err := backend.autoMigrate(); err != nil {
		t.Fatal(err)
	}
	chordtest.Run(t, backend)
}

func TestDurableChordContractLiveSQL(t *testing.T) {
	tests := []struct {
		name   string
		driver string
		dbType string
		env    string
	}{
		{name: "mysql", driver: "mysql", dbType: "mysql", env: "GKIT_MYSQL_DSN"},
		{name: "postgres", driver: "pgx", dbType: "pgsql", env: "GKIT_POSTGRES_DSN"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip(tt.env + " is required for live durable chord contract")
			}
			db, err := sql.Open(tt.driver, dsn)
			if err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() { _ = db.Close() })
			value, err := NewBackendSQLDBE(db, -1, tt.dbType, &gorm.Config{})
			if err != nil {
				t.Fatal(err)
			}
			chordtest.Run(t, value.(*BackendSQLDB))
		})
	}
}
