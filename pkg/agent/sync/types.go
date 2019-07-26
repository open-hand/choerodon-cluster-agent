package sync

type ResourceList struct {
	Resources    []string `json:"resources,omitempty"`
	ResourceType string   `json:"resourceType,omitempty"`
}
