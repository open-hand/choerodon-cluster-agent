package kubernetes

type ResourceList struct {
	Resources    []string `json:"resources,omitempty"`
	ResourceType string   `json:"resourceType,omitempty"`
}

type NodeInfo struct {
	NodeName          string `json:"nodeName,omitempty"`
	Status            string `json:"status,omitempty"`
	Type              string `json:"type,omitempty"`
	CreateTime        string `json:"createTime,omitempty"`
	CpuCapacity       string `json:"cpuCapacity,omitempty"`
	CpuAllocatable    string `json:"cpuAllocatable,omitempty"`
	PodAllocatable    string `json:"podAllocatable,omitempty"`
	PodCapacity       string `json:"podCapacity,omitempty"`
	MemoryCapacity    string `json:"memoryCapacity,omitempty"`
	MemoryAllocatable string `json:"memoryAllocatable,omitempty"`
	MemoryRequest     string `json:"memoryRequest,omitempty"`
	MemoryLimit       string `json:"memoryLimit,omitempty"`
	CpuRequest        string `json:"cpuRequest,omitempty"`
	CpuLimit          string `json:"cpuLimit,omitempty"`
	PodCount          int    `json:"podCount,omitempty"`
}
