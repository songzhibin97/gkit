package backend_db

import (
	"database/sql"
	"errors"
	"fmt"
	"math"
	"strings"
	"sync"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"

	"github.com/songzhibin97/gkit/distributed/backend"
)

const (
	defaultSQLResultExpireSeconds int64 = 3600
	expiryReadRetryLimit                = 8
)

// BackendSQLDB 支持mysql&pgsql
type BackendSQLDB struct {
	// gClient db客户端
	gClient *gorm.DB
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为s
	resultExpireMu sync.RWMutex
	resultExpire   int64
	now            func() time.Time
}

// SetResultExpire 设置结果超时时间
func (b *BackendSQLDB) SetResultExpire(expire int64) {
	b.resultExpireMu.Lock()
	b.resultExpire = normalizeSQLResultExpire(expire)
	b.resultExpireMu.Unlock()
}

func (b *BackendSQLDB) GroupTakeOver(groupID string, name string, taskIDs ...string) error {
	now := b.currentTime()
	if err := b.deleteExpiredGroupsByID(groupID, now); err != nil {
		return err
	}
	group := task.InitGroupMeta(groupID, name, b.configuredResultExpire(), taskIDs...)
	group.CreateAt = now
	err := b.gClient.Create(group).Error
	if err == nil {
		return nil
	}
	if isDuplicatedKeyError(b.gClient, err) {
		return fmt.Errorf("take over group %q: %w", groupID, backend.ErrGroupAlreadyExists)
	}
	return err
}

func isDuplicatedKeyError(db *gorm.DB, err error) bool {
	if errors.Is(err, gorm.ErrDuplicatedKey) {
		return true
	}
	translator, ok := db.Dialector.(gorm.ErrorTranslator)
	return ok && errors.Is(translator.Translate(err), gorm.ErrDuplicatedKey)
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
	for attempt := 0; attempt < expiryReadRetryLimit; attempt++ {
		var group task.GroupMeta
		err := b.gClient.Model(&task.GroupMeta{}).Where("id = ?", groupID).First(&group).Error
		if err != nil {
			return nil, err
		}
		now := b.currentTime()
		deleted, err := b.deleteExpiredGroupSnapshot(&group, now)
		if err != nil {
			return nil, err
		}
		if deleted {
			return nil, gorm.ErrRecordNotFound
		}
		if !isSQLRecordExpired(group.CreateAt, group.TTL, now) {
			return &group, nil
		}
	}
	return nil, fmt.Errorf("backend_db: group %q changed during expiry cleanup", groupID)
}

func (b *BackendSQLDB) getTaskStatus(taskIDs []string) ([]*task.Status, error) {
	statusList := make([]*task.Status, 0, len(taskIDs))
	err := b.gClient.Where("id in ?", taskIDs).Find(&statusList).Error
	if err != nil {
		return nil, err
	}
	live := make([]*task.Status, 0, len(statusList))
	now := b.currentTime()
	for _, status := range statusList {
		deleted, err := b.deleteExpiredStatusSnapshot(status, now)
		if err != nil {
			return nil, err
		}
		if deleted {
			continue
		}
		if b.isStatusExpired(status, now) {
			current, err := b.GetStatus(status.TaskID)
			if errors.Is(err, gorm.ErrRecordNotFound) {
				continue
			}
			if err != nil {
				return nil, err
			}
			live = append(live, current)
			continue
		}
		live = append(live, status)
	}
	return live, nil
}

func (b *BackendSQLDB) GroupTaskStatus(groupID string) ([]*task.Status, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return nil, err
	}
	return b.getTaskStatus(group.TaskIDs)
}

func (b *BackendSQLDB) TriggerCompleted(groupID string) (bool, error) {
	group, err := b.getGroup(groupID)
	if err != nil {
		return false, err
	}
	result := b.gClient.Model(&task.GroupMeta{}).Where(map[string]interface{}{"_id": group.ID, "lock": false}).Update("lock", true)
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
	if err := b.deleteExpiredStatusByTaskID(s.TaskID, s.CreateAt); err != nil {
		return err
	}
	// Conflict target is the DB column TaskID maps to (`id`, uniqueIndex), NOT
	// the field name. GORM writes the conflict column verbatim, so `task_id`
	// (no such column) errored on Postgres; and the column must be a UNIQUE
	// index for ON CONFLICT / ON DUPLICATE KEY to dedupe at all.
	return b.gClient.Clauses(clause.OnConflict{
		Columns:   []clause.Column{{Name: "id"}},
		DoUpdates: clause.AssignmentColumns(updateColumns),
	}).Create(s).Error
}

func (b *BackendSQLDB) SetStatePending(signature *task.Signature) error {
	return b.upsertStatus(b.newStatus(signature, task.StatePending), []string{"status"})
}

func (b *BackendSQLDB) SetStateReceived(signature *task.Signature) error {
	return b.upsertStatus(b.newStatus(signature, task.StateReceived), []string{"status"})
}

func (b *BackendSQLDB) SetStateStarted(signature *task.Signature) error {
	return b.upsertStatus(b.newStatus(signature, task.StateStarted), []string{"status"})
}

func (b *BackendSQLDB) SetStateRetry(t *task.Signature) error {
	return b.upsertStatus(b.newStatus(t, task.StateRetry), []string{"status"})
}

func (b *BackendSQLDB) SetStateSuccess(signature *task.Signature, results []*task.Result) error {
	status := b.newStatus(signature, task.StateSuccess)
	status.Results = task.Results(results)
	return b.upsertStatus(status, []string{"status", "results"})
}

func (b *BackendSQLDB) SetStateFailure(signature *task.Signature, err string) error {
	status := b.newStatus(signature, task.StateFailure)
	status.Error = err
	return b.upsertStatus(status, []string{"status", "error"})
}

func (b *BackendSQLDB) GetStatus(taskID string) (*task.Status, error) {
	for attempt := 0; attempt < expiryReadRetryLimit; attempt++ {
		var status task.Status
		err := b.gClient.Where("id = ?", taskID).First(&status).Error
		if err != nil {
			return nil, err
		}
		now := b.currentTime()
		deleted, err := b.deleteExpiredStatusSnapshot(&status, now)
		if err != nil {
			return nil, err
		}
		if deleted {
			return nil, gorm.ErrRecordNotFound
		}
		if !b.isStatusExpired(&status, now) {
			return &status, nil
		}
	}
	return nil, fmt.Errorf("backend_db: task %q changed during expiry cleanup", taskID)
}

func (b *BackendSQLDB) newStatus(signature *task.Signature, state task.State) *task.Status {
	return &task.Status{
		TaskID:   signature.ID,
		GroupID:  signature.GroupID,
		Name:     signature.Name,
		Status:   state,
		TTL:      b.configuredResultExpire(),
		CreateAt: b.currentTime(),
	}
}

func normalizeSQLResultExpire(expire int64) int64 {
	if expire == 0 {
		return defaultSQLResultExpireSeconds
	}
	return expire
}

func (b *BackendSQLDB) configuredResultExpire() int64 {
	b.resultExpireMu.RLock()
	expire := b.resultExpire
	b.resultExpireMu.RUnlock()
	return normalizeSQLResultExpire(expire)
}

func (b *BackendSQLDB) currentTime() time.Time {
	if b.now != nil {
		return b.now()
	}
	return time.Now()
}

func isSQLRecordExpired(createdAt time.Time, ttl int64, now time.Time) bool {
	ttl = normalizeSQLResultExpire(ttl)
	if ttl < 0 {
		return false
	}
	createdUnix := createdAt.Unix()
	if createdUnix > math.MaxInt64-ttl {
		return false
	}
	expiresAt := time.Unix(createdUnix+ttl, int64(createdAt.Nanosecond()))
	return !now.Before(expiresAt)
}

func (b *BackendSQLDB) isStatusExpired(status *task.Status, now time.Time) bool {
	ttl := status.TTL
	if ttl == 0 {
		// Status rows written before TTL persistence was introduced have zero in
		// the database. Apply the configured retention on read instead of
		// silently treating every legacy row as having the one-hour default.
		ttl = b.configuredResultExpire()
	}
	return isSQLRecordExpired(status.CreateAt, ttl, now)
}

func (b *BackendSQLDB) deleteExpiredStatusByTaskID(taskID string, now time.Time) error {
	var status task.Status
	result := b.gClient.Select("_id", "id", "ttl", "create_at").Where("id = ?", taskID).Limit(1).Find(&status)
	if result.Error != nil {
		return fmt.Errorf("backend_db: inspect task %q for expiry: %w", taskID, result.Error)
	}
	if result.RowsAffected == 0 {
		return nil
	}
	_, err := b.deleteExpiredStatusSnapshot(&status, now)
	return err
}

func (b *BackendSQLDB) deleteExpiredStatusSnapshot(status *task.Status, now time.Time) (bool, error) {
	if status == nil || !b.isStatusExpired(status, now) {
		return false, nil
	}
	result := b.gClient.Unscoped().Where("_id = ? AND id = ?", status.ID, status.TaskID).Delete(&task.Status{})
	if result.Error != nil {
		return false, fmt.Errorf("backend_db: delete expired task %q: %w", status.TaskID, result.Error)
	}
	return result.RowsAffected != 0, nil
}

func (b *BackendSQLDB) deleteExpiredGroupsByID(groupID string, now time.Time) error {
	var groups []*task.GroupMeta
	if err := b.gClient.Select("_id", "id", "ttl", "create_at").Where("id = ?", groupID).Find(&groups).Error; err != nil {
		return fmt.Errorf("backend_db: inspect group %q for expiry: %w", groupID, err)
	}
	for _, group := range groups {
		if _, err := b.deleteExpiredGroupSnapshot(group, now); err != nil {
			return err
		}
	}
	return nil
}

func (b *BackendSQLDB) deleteExpiredGroupSnapshot(group *task.GroupMeta, now time.Time) (bool, error) {
	if group == nil || !isSQLRecordExpired(group.CreateAt, group.TTL, now) {
		return false, nil
	}
	result := b.gClient.Unscoped().Where("_id = ? AND id = ?", group.ID, group.GroupID).Delete(&task.GroupMeta{})
	if result.Error != nil {
		return false, fmt.Errorf("backend_db: delete expired group %q: %w", group.GroupID, result.Error)
	}
	return result.RowsAffected != 0, nil
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
// NOTE: GORM's AutoMigrate will not convert pre-existing non-unique indexes on
// Status.TaskID or GroupMeta.GroupID into unique ones. Their unique indexes use
// distinct names so AutoMigrate creates them on older schemas. Index creation
// fails loudly when historical duplicates exist; this package intentionally
// does not choose and delete a surviving task or group automatically. Operators
// must reconcile duplicates before retrying migration.
// Both indexed identifiers are bounded to 191 characters so MySQL can create
// their indexes under utf8mb4; without a size GORM maps strings to LONGTEXT,
// which MySQL rejects as an index key without an explicit prefix length.
func (b *BackendSQLDB) autoMigrate() error {
	return b.gClient.AutoMigrate(
		task.GroupMeta{},
		task.Status{},
	)
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
		resultExpire: normalizeSQLResultExpire(resultExpire),
	}
	if err := b.autoMigrate(); err != nil {
		return nil, fmt.Errorf("backend_db: auto migrate: %w", err)
	}
	return b, nil
}
