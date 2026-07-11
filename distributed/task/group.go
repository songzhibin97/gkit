package task

import (
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"gorm.io/gorm"
)

type StringSlice []string

// stringSliceJSONPrefix reserves a namespace in the task_ids TEXT column for
// versioned values. Deployments must migrate historical rows beginning with
// this prefix before upgrading; the same column cannot distinguish those rows
// from versioned data. See backend_db/COMPATIBILITY.md.
const stringSliceJSONPrefix = "gkit:string-slice:v1:"

func (s *StringSlice) Scan(src interface{}) error {
	var raw string
	switch value := src.(type) {
	case nil:
		*s = nil
		return nil
	case string:
		raw = value
	case []byte:
		raw = string(value)
	default:
		return errors.New("failed to scan StringSlice field - source is not a string")
	}
	if strings.HasPrefix(raw, stringSliceJSONPrefix) {
		var elements []json.RawMessage
		if err := json.Unmarshal([]byte(strings.TrimPrefix(raw, stringSliceJSONPrefix)), &elements); err != nil {
			return fmt.Errorf("failed to scan StringSlice field - invalid versioned payload: %w", err)
		}
		if elements == nil {
			return errors.New("failed to scan StringSlice field - invalid versioned payload: null")
		}
		decoded := make([]string, len(elements))
		for index, element := range elements {
			var value *string
			if err := json.Unmarshal(element, &value); err != nil {
				return fmt.Errorf("failed to scan StringSlice field - invalid element %d: %w", index, err)
			}
			if value == nil {
				return fmt.Errorf("failed to scan StringSlice field - invalid element %d: null", index)
			}
			decoded[index] = *value
		}
		*s = decoded
		return nil
	}
	// Historical rows used comma joining. IDs containing commas in those rows
	// are already ambiguous and cannot be reconstructed without guessing.
	*s = strings.Split(raw, ",")
	return nil
}

func (s StringSlice) Value() (driver.Value, error) {
	if s == nil || len(s) == 0 {
		return nil, nil
	}
	legacy := strings.Join(s, ",")
	needsVersionedEncoding := strings.HasPrefix(legacy, stringSliceJSONPrefix)
	if !needsVersionedEncoding {
		for _, value := range s {
			if strings.Contains(value, ",") {
				needsVersionedEncoding = true
				break
			}
		}
	}
	if !needsVersionedEncoding {
		return legacy, nil
	}
	encoded, err := json.Marshal([]string(s))
	if err != nil {
		return nil, err
	}
	return stringSliceJSONPrefix + string(encoded), nil
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
