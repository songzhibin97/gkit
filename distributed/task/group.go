package task

import (
	"database/sql/driver"
	"errors"
	"strings"
	"time"

	"gorm.io/gorm"
)

type StringSlice []string

func (s *StringSlice) Scan(src interface{}) error {
	str, ok := src.([]byte)
	if !ok {
		return errors.New("failed to scan StringSlice field - source is not a string")
	}
	*s = strings.Split(string(str), ",")
	return nil
}

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		return nil, nil
	}
	return strings.Join(s, ","), nil
}

// GroupMeta 组详情
type GroupMeta struct {
	ID uint `json:"-" bson:"-" gorm:"column:_id;primarykey;comment:_id"`
	// GroupID 组的唯一标识
	GroupID string `json:"group_id" bson:"_id" gorm:"column:id;index;comment:id"`
	// 组名称
	Name string `json:"name" bson:"name" gorm:"column:name;comment:组名称"`
	// TaskIDs 接管的任务id
	TaskIDs StringSlice `json:"task_ids" bson:"task_ids" gorm:"column:task_ids;comment:接管的任务id;type:text"`
	// TriggerCompleted 是否触发完成
	TriggerCompleted bool `json:"trigger_chord" bson:"trigger_chord" gorm:"column:trigger_chord;comment:是否触发完成"`
	// Lock 是否锁定
	Lock bool `json:"lock" gorm:"column:lock;comment:锁"`
	// TTL 有效时间
	TTL int64 `json:"ttl,omitempty" bson:"ttl,omitempty" gorm:"column:ttl;comment:过期时间"`
	// CreateAt 创建时间
	CreateAt  time.Time      `json:"create_at" bson:"create_at" gorm:"column:create_at;comment:创建时间"`
	DeletedAt gorm.DeletedAt `json:"-" bson:"-" gorm:"index"`
}

func InitGroupMeta(groupID string, name string, ttl int64, taskIDs ...string) *GroupMeta {
	return &GroupMeta{
		GroupID:  groupID,
		Name:     name,
		TaskIDs:  taskIDs,
		CreateAt: time.Now().Local(),
		TTL:      ttl,
	}
}
