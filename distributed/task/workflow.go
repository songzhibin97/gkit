package task

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

// NewChain 创建链式调用任务
func NewChain(name string, signatures ...*Signature) (*Chain, error) {
	for i := len(signatures) - 1; i > 0; i-- {
		if i > 0 {
			signatures[i-1].CallbackOnSuccess = []*Signature{signatures[i]}
		}
	}
	return &Chain{Name: name, Tasks: signatures}, nil
}

// NewGroup 创建并行执行的任务组
func NewGroup(groupID string, name string, signatures ...*Signature) (*Group, error) {
	ln := len(signatures)
	for i := range signatures {
		signatures[i].GroupID = groupID
		signatures[i].GroupTaskCount = ln
	}
	return &Group{
		GroupID: groupID,
		Name:    name,
		Tasks:   signatures,
	}, nil
}

// NewGroupCallback 创建具有回调任务的个任务组
func NewGroupCallback(group *Group, name string, callback *Signature) (*GroupCallback, error) {
	for _, task := range group.Tasks {
		task.CallbackChord = callback
	}
	return &GroupCallback{
		Group:    group,
		Name:     name,
		Callback: callback,
	}, nil
}
