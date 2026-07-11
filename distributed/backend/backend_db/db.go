package backend_db

import (
	"database/sql"
	"fmt"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/songzhibin97/gkit/distributed/backend"
)

// BackendSQLDB 支持mysql&pgsql
type BackendSQLDB struct {
	// gClient db客户端
	gClient *gorm.DB
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为s
	resultExpire int64
}

const publicationAttemptColumn = "publication_attempt_id"

// statusPublicationAttempt is migration metadata for a private column on the
// task status table. It deliberately stays out of task.Status so the public
// status and backend interfaces remain compatible.
type statusPublicationAttempt struct {
	PublicationAttemptID string `gorm:"column:publication_attempt_id;size:64"`
}

// SetResultExpire 设置结果超时时间
func (b *BackendSQLDB) SetResultExpire(expire int64) {
	b.resultExpire = expire
}

func (b *BackendSQLDB) GroupTakeOver(groupID string, name string, taskIDs ...string) error {
	group := task.InitGroupMeta(groupID, name, b.resultExpire, taskIDs...)
	return b.gClient.Create(group).Error
}

func (b *BackendSQLDB) GroupCompleted(groupID string) (bool, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return false, err
	}
	status, err := b.getTaskStatus(group.TaskIDs)
	if err != nil {
		return false, err
	}
	ln := 0
	for _, t := range status {
		if !t.IsCompleted() {
			return false, nil
		}
		ln++
	}
	return len(group.TaskIDs) == ln, nil
}

func (b *BackendSQLDB) getGroup(groupID string) (*task.GroupMeta, error) {
	var group task.GroupMeta
	err := b.gClient.Model(&task.GroupMeta{}).Where("id = ?", groupID).First(&group).Error
	if err != nil {
		return nil, err
	}
	return &group, nil
}

func (b *BackendSQLDB) getTaskStatus(taskIDs []string) ([]*task.Status, error) {
	statusList := make([]*task.Status, 0, len(taskIDs))
	err := b.gClient.Where("id in ?", taskIDs).Find(&statusList).Error
	if err != nil {
		return nil, err
	}
	return statusList, nil
}

func (b *BackendSQLDB) GroupTaskStatus(groupID string) ([]*task.Status, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return nil, err
	}
	return b.getTaskStatus(group.TaskIDs)
}

func (b *BackendSQLDB) TriggerCompleted(groupID string) (bool, error) {
	result := b.gClient.Model(&task.GroupMeta{}).Where(map[string]interface{}{"id": groupID, "lock": false}).Update("lock", true)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected != 0, nil
}

// upsertStatus inserts a Status row or updates the supplied columns on
// conflict. Replaces the previous read-then-create-or-update pattern,
// which had a TOCTOU between First() and Create()/Update() — two
// concurrent SetStateX calls for the same task ID could both see
// RecordNotFound and both Create, hitting a unique-key violation; or
// Update could run on a row deleted between SELECT and UPDATE.
func (b *BackendSQLDB) upsertStatus(s *task.Status, updateColumns []string) error {
	return b.upsertStatusWithDB(b.gClient, s, updateColumns)
}

func (b *BackendSQLDB) upsertStatusWithDB(db *gorm.DB, s *task.Status, updateColumns []string) error {
	// Conflict target is the DB column TaskID maps to (`id`, uniqueIndex), NOT
	// the field name. GORM writes the conflict column verbatim, so `task_id`
	// (no such column) errored on Postgres; and the column must be a UNIQUE
	// index for ON CONFLICT / ON DUPLICATE KEY to dedupe at all.
	return db.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(s).Error
}

func (b *BackendSQLDB) SetStatePending(signature *task.Signature) error {
	return b.setStatePendingAttempt(signature, "")
}

func (b *BackendSQLDB) SetStatePendingAttempt(signature *task.Signature, attemptID string) error {
	if attemptID == "" {
		return fmt.Errorf("backend_db: empty publication attempt ID")
	}
	return b.setStatePendingAttempt(signature, attemptID)
}

func (b *BackendSQLDB) setStatePendingAttempt(signature *task.Signature, attemptID string) error {
	status := &task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   task.StatePending,
		CreateAt: time.Now(),
	}
	return b.gClient.Transaction(func(tx *gorm.DB) error {
		if err := b.upsertStatusWithDB(tx, status, []string{"status", "error"}); err != nil {
			return err
		}
		var value interface{}
		if attemptID != "" {
			value = attemptID
		}
		return tx.Model(&task.Status{}).
			Where("id = ?", signature.ID).
			Update(publicationAttemptColumn, value).Error
	})
}

func (b *BackendSQLDB) FailPendingAttempt(signature *task.Signature, attemptID, reason string) (bool, error) {
	if attemptID == "" {
		return false, nil
	}
	result := b.gClient.Model(&task.Status{}).
		Where("id = ? AND status = ? AND "+publicationAttemptColumn+" = ?", signature.ID, task.StatePending, attemptID).
		Updates(map[string]interface{}{
			"status": task.StateFailure,
			"error":  reason,
		})
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected > 0, nil
}

func (b *BackendSQLDB) SetStateReceived(signature *task.Signature) error {
	return b.upsertStatus(&task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   task.StateReceived,
		CreateAt: time.Now(),
	}, []string{"status", "error"})
}

func (b *BackendSQLDB) SetStateStarted(signature *task.Signature) error {
	return b.upsertStatus(&task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   task.StateStarted,
		CreateAt: time.Now(),
	}, []string{"status", "error"})
}

func (b *BackendSQLDB) SetStateRetry(t *task.Signature) error {
	return b.upsertStatus(&task.Status{
		TaskID:   t.ID,
		GroupID:  t.GroupID,
		Name:     t.Name,
		Status:   task.StateRetry,
		CreateAt: time.Now(),
	}, []string{"status", "error"})
}

func (b *BackendSQLDB) SetStateSuccess(signature *task.Signature, results []*task.Result) error {
	return b.upsertStatus(&task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   task.StateSuccess,
		Results:  task.Results(results),
		CreateAt: time.Now(),
	}, []string{"status", "results", "error"})
}

func (b *BackendSQLDB) SetStateFailure(signature *task.Signature, err string) error {
	return b.upsertStatus(&task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   task.StateFailure,
		Error:    err,
		CreateAt: time.Now(),
	}, []string{"status", "error"})
}

func (b *BackendSQLDB) GetStatus(taskID string) (*task.Status, error) {
	var status task.Status
	err := b.gClient.Where("id = ?", taskID).First(&status).Error
	if err != nil {
		return nil, err
	}
	return &status, nil
}

func (b *BackendSQLDB) ResetTask(taskIDs ...string) error {
	// Hard delete (Unscoped): Status has gorm.DeletedAt, so a soft delete leaves
	// the row physically present. With TaskID now a unique index, a later
	// SetStateX would upsert-conflict with that soft-deleted row and update it
	// without clearing deleted_at, so GetStatus could never see the reused task.
	return b.gClient.Unscoped().Where("id in ?", taskIDs).Delete(&task.Status{}).Error
}

func (b *BackendSQLDB) ResetGroup(groupIDs ...string) error {
	return b.gClient.Unscoped().Where("id in ?", groupIDs).Delete(&task.GroupMeta{}).Error
}

// autoMigrate creates/updates the schema.
//
// NOTE: GORM's AutoMigrate will not convert a pre-existing non-unique index on
// Status.TaskID into a unique one (it only creates the index when absent). The
// unique index is therefore given a distinct name so AutoMigrate creates it on
// older schemas — but that CREATE fails loudly if the `id` column already holds
// duplicate task IDs. Deployments upgrading from a schema that allowed
// duplicate task IDs must de-duplicate first; otherwise the upsert
// (ON CONFLICT id) cannot dedupe.
func (b *BackendSQLDB) autoMigrate() error {
	if err := b.gClient.AutoMigrate(
		task.GroupMeta{},
		task.Status{},
	); err != nil {
		return err
	}
	stmt := &gorm.Statement{DB: b.gClient}
	if err := stmt.Parse(&task.Status{}); err != nil {
		return err
	}
	migrator := b.gClient.Table(stmt.Schema.Table).Migrator()
	if migrator.HasColumn(&statusPublicationAttempt{}, "PublicationAttemptID") {
		return nil
	}
	return migrator.AddColumn(&statusPublicationAttempt{}, "PublicationAttemptID")
}

// NewBackendSQLDB constructs a SQL-backed Backend. Returns nil on failure
// (the underlying error is swallowed), preserving the original contract.
//
// Deprecated: a nil return hides why construction failed (unsupported dbType,
// the connection, or the schema migration). Use NewBackendSQLDBE, which
// returns the error.
func NewBackendSQLDB(db *sql.DB, resultExpire int64, dbType string, config *gorm.Config) backend.Backend {
	b, err := NewBackendSQLDBE(db, resultExpire, dbType, config)
	if err != nil {
		return nil
	}
	return b
}

// NewBackendSQLDBE constructs a SQL-backed Backend, returning an error
// instead of panicking when dbType is unsupported or the schema migration
// fails.
func NewBackendSQLDBE(db *sql.DB, resultExpire int64, dbType string, config *gorm.Config) (backend.Backend, error) {
	if config == nil {
		config = &gorm.Config{}
	}
	var (
		gdb *gorm.DB
		err error
	)
	switch strings.ToLower(dbType) {
	case "mysql":
		gdb, err = gorm.Open(mysql.New(mysql.Config{Conn: db}), config)
	case "pgsql":
		gdb, err = gorm.Open(postgres.New(postgres.Config{Conn: db}), config)
	default:
		return nil, fmt.Errorf("backend_db: dbType %q not supported", dbType)
	}
	if err != nil {
		return nil, fmt.Errorf("backend_db: open: %w", err)
	}
	b := &BackendSQLDB{
		gClient:      gdb,
		resultExpire: resultExpire,
	}
	if err := b.autoMigrate(); err != nil {
		return nil, fmt.Errorf("backend_db: auto migrate: %w", err)
	}
	return b, nil
}
