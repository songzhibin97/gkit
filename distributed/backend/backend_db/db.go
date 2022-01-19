package backend_db

import (
	"database/sql"
	"errors"
	"strings"
	"time"

	"github.com/songzhibin97/gkit/distributed/task"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"

	"gorm.io/gorm"

	"github.com/songzhibin97/gkit/distributed/backend"
)

// BackendSQLDB 支持mysql&pgsql
type BackendSQLDB struct {
	// gClient db客户端
	gClient *gorm.DB
	// resultExpire 数据过期时间
	// -1 代表永不过期
	// 0 会设置默认过期时间
	// 单位为ns
	resultExpire int64
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
	result := b.gClient.Debug().Model(&task.GroupMeta{}).Where("id = ? and `lock` = false", groupID).Update("`lock`", true)
	if result.Error != nil {
		return false, result.Error
	}
	return result.RowsAffected != 0, nil
}

func (b *BackendSQLDB) SetStatePending(signature *task.Signature) error {
	var status task.Status
	err := b.gClient.Where("id = ?", signature.ID).First(&status).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   signature.ID,
			GroupID:  signature.GroupID,
			Name:     signature.Name,
			Status:   task.StatePending,
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if err != nil {
		return err
	}
	// 更新
	return b.gClient.Model(&task.Status{}).Where("id = ?", signature.ID).Update("status", task.StatePending).Error
}

func (b *BackendSQLDB) SetStateReceived(signature *task.Signature) error {
	var status task.Status
	err := b.gClient.Where("id = ?", signature.ID).First(&status).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   signature.ID,
			GroupID:  signature.GroupID,
			Name:     signature.Name,
			Status:   task.StateReceived,
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if err != nil {
		return err
	}

	return b.gClient.Model(&task.Status{}).Where("id = ?", signature.ID).Update("status", task.StateReceived).Error
}

func (b *BackendSQLDB) SetStateStarted(signature *task.Signature) error {
	var status task.Status
	err := b.gClient.Where("id = ?", signature.ID).First(&status).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   signature.ID,
			GroupID:  signature.GroupID,
			Name:     signature.Name,
			Status:   task.StateStarted,
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if err != nil {
		return err
	}

	return b.gClient.Model(&task.Status{}).Where("id = ?", signature.ID).Update("status", task.StateStarted).Error
}

func (b *BackendSQLDB) SetStateRetry(t *task.Signature) error {
	var status task.Status
	err := b.gClient.Where("id = ?", t.ID).First(&status).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   t.ID,
			GroupID:  t.GroupID,
			Name:     t.Name,
			Status:   task.StateRetry,
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if err != nil {
		return err
	}

	return b.gClient.Model(&task.Status{}).Where("id = ?", t.ID).Update("status", task.StateRetry).Error
}

func (b *BackendSQLDB) SetStateSuccess(signature *task.Signature, results []*task.Result) error {
	var status task.Status
	err := b.gClient.Where("id = ?", signature.ID).First(&status).Error
	if err != nil && errors.Is(err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   signature.ID,
			GroupID:  signature.GroupID,
			Name:     signature.Name,
			Status:   task.StateSuccess,
			Results:  task.Results(results),
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if err != nil {
		return err
	}

	return b.gClient.Model(&task.Status{}).Where("id = ?", signature.ID).Updates(map[string]interface{}{"status": task.StateSuccess, "results": task.Results(results)}).Error
}

func (b *BackendSQLDB) SetStateFailure(signature *task.Signature, err string) error {
	var status task.Status
	_err := b.gClient.Where("id = ?", signature.ID).First(&status).Error
	if _err != nil && errors.Is(_err, gorm.ErrRecordNotFound) {
		// 创建
		status = task.Status{
			TaskID:   signature.ID,
			GroupID:  signature.GroupID,
			Name:     signature.Name,
			Status:   task.StateFailure,
			Error:    err,
			CreateAt: time.Now(),
		}
		return b.gClient.Create(&status).Error
	}
	if _err != nil {
		return _err
	}

	return b.gClient.Model(&task.Status{}).Where("id = ?", signature.ID).Updates(map[string]interface{}{"status": task.StateFailure, "error": err}).Error
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
	return b.gClient.Where("id in ?", taskIDs).Delete(&task.Status{}).Error
}

func (b *BackendSQLDB) ResetGroup(groupIDs ...string) error {
	return b.gClient.Where("id in ?", groupIDs).Delete(&task.GroupMeta{}).Error
}

func (b *BackendSQLDB) autoMigrate() error {
	return b.gClient.AutoMigrate(
		task.GroupMeta{},
		task.Status{},
	)
}

func NewBackendSQLDB(db *sql.DB, resultExpire int64, dbType string, config *gorm.Config) backend.Backend {
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
		panic("dbType not supported")
	}
	if err != nil {
		panic(err)
	}
	b := BackendSQLDB{
		gClient:      gdb,
		resultExpire: resultExpire,
	}
	_ = b.autoMigrate()
	return &b
}
