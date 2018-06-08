package kubernetes

type GetLogsByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
}

type ExecByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
}

type ResourceList struct {
	Resources      []string `json:"resources,omitempty"`
	ResourceType   string  `json:"resourceType,omitempty"`
}
