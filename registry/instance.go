package registry

// ServiceInstance 服务发现的实例
type ServiceInstance struct {
	// ID: 全局唯一ID
	ID string `json:"id"`
	// Name: 注册服务的名称
	Name string `json:"name"`
	// Version: 版本信息
	Version string `json:"version"`
	// Metadata: 可携带的元数据
	Metadata map[string]string `json:"metadata"`
	// Endpoints: 服务实例的端点地址
	// schema:
	//   http://127.0.0.1:8000?isSecure=false
	//   grpc://127.0.0.1:9000?isSecure=false
	Endpoints []string `json:"endpoints"`
}
