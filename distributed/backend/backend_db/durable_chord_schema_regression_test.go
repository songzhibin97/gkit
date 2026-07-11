package backend_db

import (
	"database/sql"
	"os"
	"sync"
	"testing"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
)

func TestDurableChordCurrentMasterSchemaContract(t *testing.T) {
	for _, tt := range []struct {
		name      string
		model     interface{}
		field     string
		indexName string
	}{
		{name: "group", model: &task.GroupMeta{}, field: "GroupID", indexName: "uq_group_meta_group_id"},
		{name: "status", model: &task.Status{}, field: "TaskID", indexName: "uq_status_task_id"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			parsed, err := schema.Parse(tt.model, &sync.Map{}, schema.NamingStrategy{})
			if err != nil {
				t.Fatal(err)
			}
			field := parsed.LookUpField(tt.field)
			if field == nil || field.Size != 191 {
				t.Fatalf("%s size = %v, want 191", tt.field, field)
			}
			index := parsed.LookIndex(tt.indexName)
			if index == nil || index.Class != "UNIQUE" || len(index.Fields) != 1 || index.Fields[0].DBName != "id" {
				t.Fatalf("%s index = %#v, want unique id index", tt.indexName, index)
			}
		})
	}
}

func TestDurableChordCurrentMasterSchemaLiveSQL(t *testing.T) {
	for _, tt := range []struct {
		name   string
		driver string
		dbType string
		env    string
	}{
		{name: "mysql", driver: "mysql", dbType: "mysql", env: "GKIT_MYSQL_DSN"},
		{name: "postgres", driver: "pgx", dbType: "pgsql", env: "GKIT_POSTGRES_DSN"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv(tt.env)
			if dsn == "" {
				t.Skip(tt.env + " is required for live schema verification")
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
			backend := value.(*BackendSQLDB)
			assertLiveIdentifierSchema(t, backend.gClient, &task.GroupMeta{}, "id", "uq_group_meta_group_id")
			assertLiveIdentifierSchema(t, backend.gClient, &task.Status{}, "id", "uq_status_task_id")
		})
	}
}

func assertLiveIdentifierSchema(t *testing.T, db *gorm.DB, model interface{}, columnName, indexName string) {
	t.Helper()
	columns, err := db.Migrator().ColumnTypes(model)
	if err != nil {
		t.Fatal(err)
	}
	foundColumn := false
	for _, column := range columns {
		if column.Name() != columnName {
			continue
		}
		foundColumn = true
		length, ok := column.Length()
		if !ok || length != 191 {
			t.Fatalf("%T.%s length = %d, %t; want 191", model, columnName, length, ok)
		}
	}
	if !foundColumn {
		t.Fatalf("%T.%s column not found", model, columnName)
	}
	indexes, err := db.Migrator().GetIndexes(model)
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		if index.Name() != indexName {
			continue
		}
		unique, ok := index.Unique()
		if !ok || !unique {
			t.Fatalf("%s unique = %t, %t; want true", indexName, unique, ok)
		}
		return
	}
	t.Fatalf("unique index %s not found", indexName)
}
