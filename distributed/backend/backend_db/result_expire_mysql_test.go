package backend_db

import (
	"database/sql"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
)

const sqlExpiryMySQLDSNEnv = "GKIT_TEST_MYSQL_DSN"

// These test-only models isolate the expiry contract from PR #99's pending
// production migration fix. Explicit identifier sizes let MySQL create the
// indexes while retaining the same table/column layout used by BackendSQLDB.
type sqlExpiryMySQLStatus struct {
	ID                   uint           `gorm:"column:_id;primaryKey"`
	TaskID               string         `gorm:"column:id;size:191;uniqueIndex"`
	GroupID              string         `gorm:"column:group_id"`
	Name                 string         `gorm:"column:name"`
	Status               task.State     `gorm:"column:status"`
	TTL                  int64          `gorm:"column:ttl"`
	Error                string         `gorm:"column:error"`
	Results              task.Results   `gorm:"column:results;type:text"`
	CreateAt             time.Time      `gorm:"column:create_at"`
	PublicationAttemptID string         `gorm:"column:publication_attempt_id;size:64"`
	DeletedAt            gorm.DeletedAt `gorm:"column:deleted_at;index"`
}

func (sqlExpiryMySQLStatus) TableName() string { return "statuses" }

type sqlExpiryMySQLGroup struct {
	ID               uint             `gorm:"column:_id;primaryKey"`
	GroupID          string           `gorm:"column:id;size:191;uniqueIndex"`
	Name             string           `gorm:"column:name"`
	TaskIDs          task.StringSlice `gorm:"column:task_ids;type:text"`
	TriggerCompleted bool             `gorm:"column:trigger_chord"`
	Lock             bool             `gorm:"column:lock"`
	TTL              int64            `gorm:"column:ttl"`
	CreateAt         time.Time        `gorm:"column:create_at"`
	DeletedAt        gorm.DeletedAt   `gorm:"column:deleted_at;index"`
}

func (sqlExpiryMySQLGroup) TableName() string { return "group_meta" }

func openSQLExpiryMySQL(t *testing.T) *gorm.DB {
	t.Helper()
	dsn := os.Getenv(sqlExpiryMySQLDSNEnv)
	if dsn == "" {
		t.Skipf("set %s to run the isolated MySQL expiry contract", sqlExpiryMySQLDSNEnv)
	}
	sqlDB, err := sql.Open("mysql", dsn)
	if err != nil {
		t.Fatalf("open MySQL: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if err := sqlDB.Ping(); err != nil {
		t.Fatalf("ping MySQL: %v", err)
	}
	gdb, err := gorm.Open(mysql.New(mysql.Config{Conn: sqlDB}), &gorm.Config{})
	if err != nil {
		t.Fatalf("open GORM MySQL: %v", err)
	}
	var database string
	if err := gdb.Raw("SELECT DATABASE()").Scan(&database).Error; err != nil {
		t.Fatalf("read MySQL database name: %v", err)
	}
	if !strings.HasSuffix(strings.ToLower(database), "_test") {
		t.Fatalf("%s must select an isolated database ending in _test, got %q", sqlExpiryMySQLDSNEnv, database)
	}
	if err := gdb.Migrator().DropTable(&task.Status{}, &task.GroupMeta{}); err != nil {
		t.Fatalf("drop MySQL test tables: %v", err)
	}
	t.Cleanup(func() { _ = gdb.Migrator().DropTable(&task.Status{}, &task.GroupMeta{}) })
	return gdb
}

func TestSQLResultExpireMySQLGroupStatusContract(t *testing.T) {
	base := time.Date(2026, 7, 11, 12, 0, 0, 0, time.UTC)
	clock := &sqlExpiryClock{now: base}
	b := &BackendSQLDB{gClient: openSQLExpiryMySQL(t), now: clock.Now}
	b.SetResultExpire(20)
	if err := b.gClient.AutoMigrate(&sqlExpiryMySQLStatus{}, &sqlExpiryMySQLGroup{}); err != nil {
		t.Fatalf("create isolated expiry schema: %v", err)
	}
	if err := b.GroupTakeOver("group", "group", "member"); err != nil {
		t.Fatalf("GroupTakeOver: %v", err)
	}
	b.SetResultExpire(5)
	if err := b.SetStatePending(&task.Signature{ID: "member", GroupID: "group", Name: "task"}); err != nil {
		t.Fatalf("SetStatePending: %v", err)
	}
	stored := readStoredStatus(t, b, "member")

	clock.Set(base.Add(5 * time.Second))
	statuses, err := b.GroupTaskStatus("group")
	if err != nil {
		t.Fatalf("GroupTaskStatus: %v", err)
	}
	if len(statuses) != 0 {
		t.Fatalf("statuses = %v, want expired member omitted", statuses)
	}
	if count := countUnscopedRows(t, b.gClient, &task.Status{}, "_id = ?", stored.ID); count != 0 {
		t.Fatalf("physical expired member rows = %d, want 0", count)
	}
}
