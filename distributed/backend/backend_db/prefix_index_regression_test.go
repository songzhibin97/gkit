package backend_db

import (
	"database/sql"
	"fmt"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"os"
	"testing"
	"time"
)

func TestIssue167PostgresPrefixUniqueID(t *testing.T) {
	dsn := os.Getenv("GKIT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("#167 live regression requires GKIT_POSTGRES_DSN; owned fixture required before acceptance")
	}
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := db.Close(); err != nil {
			t.Error(err)
		}
	})
	original, err := NewBackendSQLDBE(db, -1, "pgsql", nil)
	if err != nil {
		t.Fatal(err)
	}
	id := fmt.Sprintf("issue167-original-%d", time.Now().UnixNano())
	if err := original.SetStateSuccess(&task.Signature{ID: id}, []*task.Result{{Type: "int64", Value: int64(7)}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		if err := original.ResetTask(id); err != nil {
			t.Error(err)
		}
	})
	prefix := fmt.Sprintf("issue167_%d_", time.Now().UnixNano())
	value, err := NewBackendSQLDBE(db, -1, "pgsql", &gorm.Config{NamingStrategy: schema.NamingStrategy{TablePrefix: prefix}})
	if err != nil {
		t.Fatal(err)
	}
	b := value.(*BackendSQLDB)
	t.Cleanup(func() {
		if err := b.gClient.Migrator().DropTable(&task.Status{}, &task.GroupMeta{}); err != nil {
			t.Error(err)
		}
	})
	for _, model := range []interface{}{&task.Status{}, &task.GroupMeta{}} {
		indexes, err := b.gClient.Migrator().GetIndexes(model)
		if err != nil {
			t.Fatal(err)
		}
		found := false
		for _, index := range indexes {
			unique, ok := index.Unique()
			columns := index.Columns()
			if ok && unique && len(columns) == 1 && columns[0] == "id" {
				found = true
			}
		}
		if !found {
			t.Errorf("%T has no unique id index", model)
		}
	}
	sig := &task.Signature{ID: "prefix-task"}
	if err := b.SetStatePending(sig); err != nil {
		t.Fatal(err)
	}
	if err := b.SetStateSuccess(sig, []*task.Result{{Type: "int64", Value: int64(7)}}); err != nil {
		t.Fatal(err)
	}
	if _, err := b.GetStatus(sig.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.GroupTakeOver("prefix-group", "ordered", sig.ID); err != nil {
		t.Fatal(err)
	}
	if err := b.GroupTakeOver("prefix-group", "duplicate", sig.ID); err == nil {
		t.Fatal("duplicate group accepted")
	}
	if err := b.autoMigrate(); err != nil {
		t.Fatalf("repeat migration: %v", err)
	}
	old, err := original.GetStatus(id)
	if err != nil || old.Status != task.StateSuccess {
		t.Fatalf("original table changed: %v %v", old, err)
	}
	// Historical duplicates must stop migration, without deleting either row.
	indexes, err := b.gClient.Migrator().GetIndexes(&task.Status{})
	if err != nil {
		t.Fatal(err)
	}
	for _, index := range indexes {
		unique, ok := index.Unique()
		columns := index.Columns()
		if ok && unique && len(columns) == 1 && columns[0] == "id" {
			if err := b.gClient.Migrator().DropIndex(&task.Status{}, index.Name()); err != nil {
				t.Fatal(err)
			}
		}
	}
	for i := 0; i < 2; i++ {
		if err := b.gClient.Create(&task.Status{TaskID: "historical-duplicate", TTL: -1}).Error; err != nil {
			t.Fatal(err)
		}
	}
	if err := b.autoMigrate(); err == nil {
		t.Fatal("migration accepted historical duplicate IDs")
	}
	var count int64
	if err := b.gClient.Model(&task.Status{}).Where("id = ?", "historical-duplicate").Count(&count).Error; err != nil {
		t.Fatal(err)
	}
	if count != 2 {
		t.Fatalf("historical rows after failed migration = %d", count)
	}

}
