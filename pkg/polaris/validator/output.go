// Copyright 2019 FairwindsOps Inc
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package validator

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/config"
)

const (
	// PolarisOutputVersion is the version of the current output structure
	PolarisOutputVersion = "1.0"
)

// AuditData contains all the data from a full Polaris audit
type AuditData struct {
	PolarisOutputVersion string             `json:"polarisOutputVersion"`
	AuditTime            string             `json:"auditTime"`
	SourceType           string             `json:"sourceType"`
	SourceName           string             `json:"sourceName"`
	DisplayName          string             `json:"displayName"`
	ClusterInfo          ClusterInfo        `json:"clusterInfo"`
	Results              []ControllerResult `json:"results"`
}

// ClusterInfo contains Polaris results as well as some high-level stats
type ClusterInfo struct {
	Version                string `json:"version"`
	Nodes                  int    `json:"nodes"`
	Pods                   int    `json:"pods"`
	Namespaces             int    `json:"namespaces"`
	Deployments            int    `json:"deployments"`
	StatefulSets           int    `json:"statefulSets"`
	DaemonSets             int    `json:"daemonSets"`
	Jobs                   int    `json:"jobs"`
	CronJobs               int    `json:"cronCobs"`
	ReplicationControllers int    `json:"replicationControllers"`
}

// ResultMessage is the result of a given check
type ResultMessage struct {
	ID       string          `json:"id"`
	Message  string          `json:"message"`
	Success  bool            `json:"success"`
	Severity config.Severity `json:"severity"`
	Category string          `json:"category"`
}

// ResultSet contiains the results for a set of checks
type ResultSet map[string]ResultMessage

// ControllerResult provides results for a controller
type ControllerResult struct {
	Name      string    `json:"name"`
	Namespace string    `json:"namespace"`
	Kind      string    `json:"kind"`
	Results   ResultSet `json:"results"`
	PodResult PodResult `json:"podResult"`
}

// PodResult provides a list of validation messages for each pod.
type PodResult struct {
	Name             string            `json:"name"`
	Results          ResultSet         `json:"results"`
	ContainerResults []ContainerResult `json:"containerResults"`
}

// ContainerResult provides a list of validation messages for each container.
type ContainerResult struct {
	Name    string    `json:"name"`
	Results ResultSet `json:"results"`
}
