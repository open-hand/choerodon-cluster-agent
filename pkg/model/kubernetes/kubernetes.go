package kubernetes

type GetLogsByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

type ExecByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

type ScalePodRequest struct {
	DeploymentName string `json:"deploymentName,omitempty"`
	Count          int    `json:"count,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
}

type ResourceList struct {
	Resources    []string `json:"resources,omitempty"`
	ResourceType string   `json:"resourceType,omitempty"`
}

type NodeInfo struct {
	NodeName      string `json:"nodeName,omitempty"`
	Status        string `json:"status,omitempty"`
	Type          string `json:"type,omitempty"`
	CreateTime    string `json:"createTime,omitempty"`
	CpuRequest    string `json:"cpuRequest,omitempty"`
	CpuLimit      string `json:"cpuLimit,omitempty"`
	PodCount      string `json:"podCount,omitempty"`
	PodLimit      string `json:"podLimit,omitempty"`
	MemoryRequest string `json:"memoryRequest,omitempty"`
	MemoryLimit   string `json:"MemoryLimit,omitempty"`
}
