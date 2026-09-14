package backend_db

import (
	"database/sql"
	"fmt"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/gorm"
	"gorm.io/gorm/schema"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestIssue167PostgresUniqueIndexConvergence(t *testing.T) {
	dsn := os.Getenv("GKIT_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("#167 live migration requires GKIT_POSTGRES_DSN; owned fixture required before acceptance")
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
	prefix := fmt.Sprintf("issue167_converge_%d_", time.Now().UnixNano())
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
	// Both real catalog queries execute before either initializer can CREATE.
	// The callbacks only schedule the race; no catalog results are replaced.
	entered := make(chan struct{}, 2)
	release := make(chan struct{})
	var reads atomic.Int32
	if err := b.gClient.Callback().Row().After("gorm:row").Register("issue167-catalog-barrier", func(tx *gorm.DB) {
		if strings.Contains(tx.Statement.SQL.String(), "to_regclass") && reads.Add(1) <= 2 {
			entered <- struct{}{}
			<-release
		}
	}); err != nil {
		t.Fatal(err)
	}
	results := make(chan error, 2)
	for i := 0; i < 2; i++ {
		go func() { results <- b.ensurePostgresUniqueID(&task.Status{}) }()
	}
	for i := 0; i < 2; i++ {
		select {
		case <-entered:
		case <-time.After(5 * time.Second):
			close(release)
			t.Fatal("catalog barrier not reached")
		}
	}
	close(release)
	for i := 0; i < 2; i++ {
		if err := <-results; err != nil {
			t.Errorf("concurrent initializer: %v", err)
		}
	}
	if err := b.gClient.Callback().Row().Remove("issue167-catalog-barrier"); err != nil {
		t.Fatal(err)
	}
	if err := b.SetStatePending(&task.Signature{ID: "after-concurrent-migration"}); err != nil {
		t.Fatal(err)
	}
	// A unique key with included payload columns is already sufficient.
	stmt := &gorm.Statement{DB: b.gClient}
	if err := stmt.Parse(&task.Status{}); err != nil {
		t.Fatal(err)
	}
	dynamic := b.gClient.NamingStrategy.IndexName(stmt.Schema.Table, "gkit_unique_id")
	if err := b.gClient.Migrator().DropIndex(&task.Status{}, dynamic); err != nil {
		t.Fatal(err)
	}
	includeIndex := prefix + "covering_id"
	if err := b.gClient.Exec("CREATE UNIQUE INDEX " + stmt.Quote(includeIndex) + " ON " + stmt.Quote(stmt.Schema.Table) + " (id) INCLUDE (name)").Error; err != nil {
		t.Fatal(err)
	}
	if err := b.ensurePostgresUniqueID(&task.Status{}); err != nil {
		t.Fatal(err)
	}
	if b.gClient.Migrator().HasIndex(&task.Status{}, dynamic) {
		t.Error("created redundant unique index despite INCLUDE key")
	}
	// An index with our generated name on another table is not convergence.
	if err := b.gClient.Migrator().DropIndex(&task.Status{}, includeIndex); err != nil {
		t.Fatal(err)
	}
	groupStmt := &gorm.Statement{DB: b.gClient}
	if err := groupStmt.Parse(&task.GroupMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.gClient.Exec("CREATE UNIQUE INDEX " + stmt.Quote(dynamic) + " ON " + groupStmt.Quote(groupStmt.Schema.Table) + " (id)").Error; err != nil {
		t.Fatal(err)
	}
	if err := b.ensurePostgresUniqueID(&task.Status{}); err == nil {
		t.Fatal("unrelated table's index accepted as convergence")
	}

}
