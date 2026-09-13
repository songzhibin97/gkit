package backend_db

import (
	"database/sql"
	"errors"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

func TestIssue167PostgresDeferrableArbiters(t *testing.T) {
	for _, tt := range []struct {
		name, columns, table string
		reject               bool
	}{{"matching-id", "id", "statuses", true}, {"unrelated-multicolumn", "id, name", "statuses", false}, {"unrelated-column", "name", "statuses", false}, {"group-id", "id", "group_meta", false}} {
		t.Run(tt.name, func(t *testing.T) {
			dsn := os.Getenv("GKIT_POSTGRES_DSN")
			if dsn == "" {
				t.Skip("#167 live migration requires GKIT_POSTGRES_DSN; owned fixture required before acceptance")
			}
			db, err := sql.Open("pgx", dsn)
			if err != nil {
				t.Fatal(err)
			}
			db.SetMaxOpenConns(1)
			db.SetMaxIdleConns(1)
			t.Cleanup(func() {
				if err := db.Close(); err != nil {
					t.Error(err)
				}
			})
			scratch := fmt.Sprintf("issue167_deferrable_%d", time.Now().UnixNano())
			if _, err := db.Exec("CREATE SCHEMA " + scratch); err != nil {
				t.Fatal(err)
			}
			t.Cleanup(func() {
				if _, err := db.Exec("SET search_path TO public"); err != nil {
					t.Error(err)
				}
				if _, err := db.Exec("DROP SCHEMA " + scratch + " CASCADE"); err != nil {
					t.Error(err)
				}
			})
			if _, err := db.Exec("SET search_path TO " + scratch); err != nil {
				t.Fatal(err)
			}
			config := func() *gorm.Config { return &gorm.Config{Logger: logger.Default.LogMode(logger.Silent)} }
			original, err := NewBackendSQLDBE(db, -1, "pgsql", config())
			if err != nil {
				t.Fatal(err)
			}
			sig := &task.Signature{ID: "existing-result", Name: "existing-name"}
			if err := original.SetStateSuccess(sig, []*task.Result{{Type: "int64", Value: int64(7)}}); err != nil {
				t.Fatal(err)
			}
			constraint := "issue167_other_unique"
			if tt.reject || tt.table == "group_meta" {
				constraint = "uq_status_task_id"
				if tt.table == "group_meta" {
					constraint = "uq_group_meta_group_id"
				}
				if _, err := db.Exec("DROP INDEX " + constraint); err != nil {
					t.Fatal(err)
				}
			}
			if _, err := db.Exec("ALTER TABLE " + tt.table + " ADD CONSTRAINT " + constraint + " UNIQUE (" + tt.columns + ") DEFERRABLE INITIALLY IMMEDIATE"); err != nil {
				t.Fatal(err)
			}
			if tt.reject {
				if _, err := db.Exec("ALTER TABLE statuses ADD CONSTRAINT uni_statuses_id UNIQUE(id)"); err != nil {
					t.Fatal(err)
				}
			}
			if tt.table == "group_meta" {
				if _, err := db.Exec("ALTER TABLE group_meta ADD CONSTRAINT uni_group_meta_id UNIQUE(id)"); err != nil {
					t.Fatal(err)
				}
			}
			if tt.name == "unrelated-column" {
				// GORM expects its own immediate constraint name when it removes
				// model-undeclared uniqueness; keep that independent behavior valid.
				if _, err := db.Exec("ALTER TABLE statuses ADD CONSTRAINT uni_statuses_name UNIQUE(name)"); err != nil {
					t.Fatal(err)
				}
			}
			b, err := NewBackendSQLDBE(db, -1, "pgsql", config())
			if tt.reject {
				if err == nil {
					t.Error("constructor accepted matching deferrable id arbiter")
					t.Logf("subsequent write: %v", b.SetStatePending(&task.Signature{ID: "new-result", Name: "new-name"}))
				} else if !strings.Contains(err.Error(), "deferrable") {
					t.Errorf("non-specific initialization error: %v", err)
				}
				var count int
				if err := db.QueryRow("SELECT count(*) FROM pg_constraint WHERE conrelid='statuses'::regclass AND conname IN ('uq_status_task_id','uni_statuses_id')").Scan(&count); err != nil {
					t.Fatal(err)
				}
				if count != 2 {
					t.Errorf("constructor changed existing constraints: count=%d want 2", count)
				}
			} else {
				if err != nil {
					t.Fatalf("unrelated deferrable constraint rejected: %v", err)
				}
				next := &task.Signature{ID: "new-result", Name: "new-name"}
				if err := b.SetStatePending(next); err != nil {
					t.Fatal(err)
				}
				if err := b.SetStateStarted(next); err != nil {
					t.Fatal(err)
				}
			}
			if tt.table == "group_meta" {
				if err := b.GroupTakeOver("new-group", "group", "member"); err != nil {
					t.Fatal(err)
				}
				if err := b.GroupTakeOver("new-group", "duplicate", "member"); !errors.Is(err, backend.ErrGroupAlreadyExists) {
					t.Fatalf("group uniqueness: %v", err)
				}
				var immediate int
				if err := db.QueryRow("SELECT count(*) FROM pg_index WHERE indrelid='group_meta'::regclass AND indisunique AND indimmediate AND NOT indisprimary").Scan(&immediate); err != nil {
					t.Fatal(err)
				}
				if immediate != 0 {
					t.Errorf("constructor added unnecessary immediate group key: %d", immediate)
				}
			}
			var preserved bool
			if err := db.QueryRow("SELECT condeferrable FROM pg_constraint WHERE conrelid=to_regclass($1) AND conname=$2", tt.table, constraint).Scan(&preserved); err != nil || !preserved {
				t.Fatalf("deferrable constraint changed: %t %v", preserved, err)
			}
			existing, err := original.GetStatus(sig.ID)
			if err != nil || len(existing.Results) != 1 || existing.Results[0].Value != int64(7) {
				t.Fatalf("original result changed: %v %v", existing, err)
			}
		})
	}
}
