package task

// Arg task中的参数
type Arg struct {
	Type  string      `json:"type" bson:"type"`
	Key   string      `json:"key" bson:"key"`
	Value interface{} `json:"value" bson:"value"`
}
