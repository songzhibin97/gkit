package backend_db

import (
	"database/sql"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/songzhibin97/gkit/distributed/backend"
	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const mysqlTestDSNEnv = "GKIT_TEST_MYSQL_DSN"

func openMySQLTestDB(t *testing.T, translateError bool) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(mysqlTestDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run MySQL migration tests", mysqlTestDSNEnv)
	}

	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}

	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{TranslateError: translateError})
	if err != nil {
		t.Fatalf("open GORM MySQL: %v", err)
	}
	var database string
	if err := gdb.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil {
		t.Fatalf("read MySQL database name: %v", err)
	}
	if !strings.HasSuffix(strings.ToLower(database), "_test") {
		t.Fatalf("%s must select an isolated database whose name ends in _test, got %q", mysqlTestDSNEnv, database)
	}
	return gdb
}

func resetMySQLTables(t *testing.T, gdb *gorm.DB) {
	t.Helper()
	if err := gdb.Migrator().DropTable(&task.Status{}, &task.GroupMeta{}); err != nil {
		t.Fatalf("drop MySQL test tables: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Migrator().DropTable(&task.Status{}, &task.GroupMeta{}) })
}

func TestMySQLUniqueIdentifierMigrationAndConflicts(t *testing.T) {
	for _, translateError := range []bool{false, true} {
		name := "translate_error_off"
		if translateError {
			name = "translate_error_on"
		}
		t.Run(name, func(t *testing.T) {
			gdb := openMySQLTestDB(t, translateError)
			resetMySQLTables(t, gdb)
			b := &BackendSQLDB{gClient: gdb}
			if err := b.autoMigrate(); err != nil {
				t.Fatalf("auto migrate: %v", err)
			}

			groupID := strings.Repeat("g", 191)
			if err := b.GroupTakeOver(groupID, "first", "task-1"); err != nil {
				t.Fatalf("first GroupTakeOver: %v", err)
			}
			err := b.GroupTakeOver(groupID, "second", "task-2")
			if !errors.Is(err, backend.ErrGroupAlreadyExists) {
				t.Fatalf("second GroupTakeOver error = %v, want ErrGroupAlreadyExists", err)
			}

			taskID := strings.Repeat("t", 191)
			signature := &task.Signature{ID: taskID, GroupID: groupID, Name: "task"}
			if err := b.SetStatePending(signature); err != nil {
				t.Fatalf("SetStatePending: %v", err)
			}
			if err := b.SetStateStarted(signature); err != nil {
				t.Fatalf("SetStateStarted: %v", err)
			}
			var count int64
			if err := gdb.Model(&task.Status{}).Where("id = ?", taskID).Count(&count).Error; err != nil {
				t.Fatalf("count status rows: %v", err)
			}
			if count != 1 {
				t.Fatalf("rows for task ID = %d, want 1", count)
			}
			status, err := b.GetStatus(taskID)
			if err != nil {
				t.Fatalf("GetStatus: %v", err)
			}
			if status.Status != task.StateStarted {
				t.Fatalf("status = %v, want StateStarted", status.Status)
			}
		})
	}
}

func TestMySQLAutoMigrateRejectsHistoricalDuplicateGroupsWithoutDeleting(t *testing.T) {
	gdb := openMySQLTestDB(t, false)
	resetMySQLTables(t, gdb)
	if err := gdb.AutoMigrate(&legacyGroupMeta{}); err != nil {
		t.Fatalf("create legacy schema: %v", err)
	}
	for index := 0; index < 2; index++ {
		if err := gdb.Create(&legacyGroupMeta{GroupID: "duplicate-group"}).Error; err != nil {
			t.Fatalf("insert legacy duplicate %d: %v", index, err)
		}
	}

	b := &BackendSQLDB{gClient: gdb}
	if err := b.autoMigrate(); err == nil {
		t.Fatal("autoMigrate succeeded with historical duplicate group IDs")
	}

	var count int64
	if err := gdb.Table("group_meta").Where("id = ?", "duplicate-group").Count(&count).Error; err != nil {
		t.Fatalf("count historical groups: %v", err)
	}
	if count != 2 {
		t.Fatalf("historical duplicate rows after failed migration = %d, want 2", count)
	}
}
