package task

import (
	"bytes"
	"encoding/json"
	"time"

	"github.com/songzhibin97/gkit/tools/deepcopy"

	"github.com/songzhibin97/gkit/options"
)

type Signature struct {
	// ID 任务唯一id,要保证多实例中id唯一
	ID string `json:"id" bson:"_id"`
	// Name 任务名称
	Name string `json:"name" bson:"name"`
	// GroupID 多集群中组id
	GroupID string `json:"group_id" bson:"groupID"`
	// GroupTaskCount 组中任务计数
	GroupTaskCount int `json:"group_task_count" bson:"group_task_count"`
	// Priority 任务优先级
	Priority uint8 `json:"priority" bson:"priority"`
	// RetryCount 重试次数
	RetryCount int `json:"retry_count" bson:"retry_count"`
	// RetryInterval 重试间隔时间，单位为秒
	RetryInterval int `json:"retry_timeout" bson:"retry_timeout"`
	// StopTaskDeletionOnError 任务出错后删除
	StopTaskDeletionOnError bool `json:"stop_task_deletion_on_error" bson:"stop_task_deletion_on_error"`
	// IgnoreNotRegisteredTask 忽略未注册的任务
	IgnoreNotRegisteredTask bool `json:"not_registered" bson:"not_registered"`
	// Router 路由
	Router string `json:"router" bson:"router"`
	// Args 携带参数
	Args []Arg `json:"args" bson:"args"`
	// MetaSafe 安全的Meta
	MetaSafe bool `json:"meta_safe" bson:"meta_safe"`
	// Meta 携带原信息
	Meta *Meta `json:"meta" bson:"meta"`
	// ETA 延时任务
	ETA *time.Time `json:"eta" bson:"eta"`
	// CallbackChord 组任务回调
	CallbackChord *Signature `json:"callback_chord" bson:"callback_chord"`
	// CallbackOnSuccess 任务成功后回调
	CallbackOnSuccess []*Signature `json:"callback_on_success" bson:"callback_on_success"`
	// CallbackOnError 任务失败后回调
	CallbackOnError []*Signature `json:"callback_on_error" bson:"callback_on_error"`
}

// NewSignature 创建Signature
func NewSignature(id string, name string, options ...options.Option) *Signature {
	task := &Signature{
		ID:                      id,
		Name:                    name,
		GroupID:                 "-",
		Priority:                0,
		RetryCount:              3,
		RetryInterval:           60,
		StopTaskDeletionOnError: false,
		IgnoreNotRegisteredTask: false,
		Router:                  "",
		Args:                    nil,
		MetaSafe:                true,
		Meta:                    NewMeta(true),
		ETA:                     nil,
		CallbackChord:           nil,
		CallbackOnSuccess:       nil,
		CallbackOnError:         nil,
	}
	for _, option := range options {
		option(task)
	}
	normalizeSignatureMeta(task, make(map[*Signature]struct{}))
	return task
}

// UnmarshalJSON restores signature metadata maps and reapplies MetaSafe after
// decoding. UseNumber preserves the existing controller decoder's numeric
// behavior for interface-valued task arguments.
func (s *Signature) UnmarshalJSON(data []byte) error {
	type signatureAlias Signature
	var decoded signatureAlias
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return err
	}
	*s = Signature(decoded)
	normalizeSignatureMeta(s, make(map[*Signature]struct{}))
	return nil
}

func normalizeSignatureMeta(signature *Signature, visited map[*Signature]struct{}) {
	if signature == nil {
		return
	}
	if _, ok := visited[signature]; ok {
		return
	}
	visited[signature] = struct{}{}
	if signature.Meta == nil {
		signature.Meta = NewMeta(signature.MetaSafe)
	} else {
		signature.Meta.normalize(signature.MetaSafe)
	}
	for _, callback := range signature.CallbackOnSuccess {
		normalizeSignatureMeta(callback, visited)
	}
	for _, callback := range signature.CallbackOnError {
		normalizeSignatureMeta(callback, visited)
	}
	normalizeSignatureMeta(signature.CallbackChord, visited)
}

func CopySignatures(signatures ...*Signature) []*Signature {
	sigs := make([]*Signature, len(signatures))
	for index, signature := range signatures {
		sigs[index] = CopySignature(signature)
	}
	return sigs
}

func CopySignature(signature *Signature) *Signature {
	if signature == nil {
		return &Signature{}
	}
	cloned, ok := deepcopy.Clone(signature).(*Signature)
	if !ok || cloned == nil {
		return &Signature{}
	}
	cloneSignatureMeta(signature, cloned, make(map[*Signature]*Signature))
	return cloned
}

func cloneSignatureMeta(source, destination *Signature, visited map[*Signature]*Signature) {
	if source == nil || destination == nil {
		return
	}
	if _, ok := visited[source]; ok {
		return
	}
	visited[source] = destination
	destination.Meta = source.Meta.clone(source.MetaSafe)

	for index, callback := range source.CallbackOnSuccess {
		if index < len(destination.CallbackOnSuccess) {
			cloneSignatureMeta(callback, destination.CallbackOnSuccess[index], visited)
		}
	}
	for index, callback := range source.CallbackOnError {
		if index < len(destination.CallbackOnError) {
			cloneSignatureMeta(callback, destination.CallbackOnError[index], visited)
		}
	}
	cloneSignatureMeta(source.CallbackChord, destination.CallbackChord, visited)
}
