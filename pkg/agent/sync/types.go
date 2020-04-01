package sync

type ResourceList struct {
	Resources    []string `json:"resources"`
	ResourceType string   `json:"resourceType,omitempty"`
}
