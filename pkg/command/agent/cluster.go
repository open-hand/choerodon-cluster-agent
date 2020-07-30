package agent

type ClusterInfo struct {
	Version    string `json:"version"`
	Pods       int    `json:"pods"`
	Namespaces int    `json:"namespaces"`
	Nodes      int    `json:"nodes"`
	ClusterId  string `json:"clusterId"`
}
