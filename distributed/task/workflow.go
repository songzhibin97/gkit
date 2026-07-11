package task

import (
	"errors"
	"fmt"
)

// ErrInvalidWorkflow is returned when a workflow cannot execute safely.
var ErrInvalidWorkflow = errors.New("invalid workflow")

// Chain 创建链式调用的任务
type Chain struct {
	Name  string
	Tasks []*Signature
}

// Group 创建并行执行的任务组
type Group struct {
	Name    string
	GroupID string
	Tasks   []*Signature
}

// GroupCallback 具有回调任务的任务组
type GroupCallback struct {
	Name     string
	Group    *Group
	Callback *Signature
}

// GetTaskIDs 获取组任务的所有ID
func (g *Group) GetTaskIDs() []string {
	ids := make([]string, 0, len(g.Tasks))
	for _, task := range g.Tasks {
		ids = append(ids, task.ID)
	}
	return ids
}

// ValidateChain validates a chain before it is wired or sent.
func ValidateChain(chain *Chain) error {
	if chain == nil {
		return fmt.Errorf("%w: nil chain", ErrInvalidWorkflow)
	}
	return validateWorkflowTasks("chain", chain.Tasks)
}

// ValidateGroup validates a group before it is wired or sent.
func ValidateGroup(group *Group) error {
	if group == nil {
		return fmt.Errorf("%w: nil group", ErrInvalidWorkflow)
	}
	return validateWorkflowTasks("group", group.Tasks)
}

// ValidateGroupCallback validates a group callback before it is wired or sent.
func ValidateGroupCallback(groupCallback *GroupCallback) error {
	if groupCallback == nil {
		return fmt.Errorf("%w: nil group callback", ErrInvalidWorkflow)
	}
	if err := ValidateGroup(groupCallback.Group); err != nil {
		return err
	}
	if groupCallback.Callback == nil {
		return fmt.Errorf("%w: nil group callback task", ErrInvalidWorkflow)
	}
	return nil
}

func validateWorkflowTasks(kind string, signatures []*Signature) error {
	if len(signatures) == 0 {
		return fmt.Errorf("%w: empty %s", ErrInvalidWorkflow, kind)
	}
	for index, signature := range signatures {
		if signature == nil {
			return fmt.Errorf("%w: nil %s task at index %d", ErrInvalidWorkflow, kind, index)
		}
	}
	return nil
}

// NewChain 创建链式调用任务
func NewChain(name string, signatures ...*Signature) (*Chain, error) {
	chain := &Chain{Name: name, Tasks: signatures}
	if err := ValidateChain(chain); err != nil {
		return nil, err
	}
	for i := len(signatures) - 1; i > 0; i-- {
		if i > 0 {
			signatures[i-1].CallbackOnSuccess = []*Signature{signatures[i]}
		}
	}
	return chain, nil
}

// NewGroup 创建并行执行的任务组
func NewGroup(groupID string, name string, signatures ...*Signature) (*Group, error) {
	group := &Group{GroupID: groupID, Name: name, Tasks: signatures}
	if err := ValidateGroup(group); err != nil {
		return nil, err
	}
	ln := len(signatures)
	for i := range signatures {
		signatures[i].GroupID = groupID
		signatures[i].GroupTaskCount = ln
	}
	return group, nil
}

// NewGroupCallback 创建具有回调任务的个任务组
func NewGroupCallback(group *Group, name string, callback *Signature) (*GroupCallback, error) {
	groupCallback := &GroupCallback{Group: group, Name: name, Callback: callback}
	if err := ValidateGroupCallback(groupCallback); err != nil {
		return nil, err
	}
	for _, task := range group.Tasks {
		task.CallbackChord = callback
	}
	return groupCallback, nil
}
