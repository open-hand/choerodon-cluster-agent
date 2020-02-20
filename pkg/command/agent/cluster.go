package agent

type ClusterInfo struct {
	Version    string `json:"version"`
	Pods       int    `json:"pods"`
	Namespaces int    `json:"namespaces"`
	Nodes      int    `json:"nodes"`
	ClusterId  int    `json:"clusterId"`
}
