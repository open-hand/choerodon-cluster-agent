package helm

import (
	core_v1 "k8s.io/api/core/v1"
)

type InstallReleaseRequest struct {
	RepoURL          string                         `json:"repoURL,omitempty"`
	ChartName        string                         `json:"chartName,omitempty"`
	ChartVersion     string                         `json:"chartVersion,omitempty"`
	Values           string                         `json:"values,omitempty"`
	ReleaseName      string                         `json:"releaseName,omitempty"`
	Commit           string                         `json:"commit,omitempty"`
	Command          int                            `json:"command,omitempty"`
	Namespace        string                         `json:"namespace,omitempty"`
	AppServiceId     int64                          `json:"appServiceId,omitempty"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type TestReleaseRequest struct {
	RepoURL          string                         `json:"repoURL,omitempty"`
	ChartName        string                         `json:"chartName,omitempty"`
	ChartVersion     string                         `json:"chartVersion,omitempty"`
	Values           string                         `json:"values,omitempty"`
	ReleaseName      string                         `json:"releaseName,omitempty"`
	Label            string                         `json:"label,omitempty"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type TestStatusResponse struct {
	ReleaseName   string `json:"releaseName,omitempty"`
	Pod           string `json:"pod,omitempty"`
	ReleaseStatus string `json:"releaseStatus,omitempty"`
}

type TestJobFinished struct {
	Succeed bool   `json:"succeed,omitempty"`
	Log     string `json:"log,omitempty"`
}

type TestReleaseResponse struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type TestReleaseStatus struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Status      string `json:"status,omitempty"`
}

type Release struct {
	Name         string             `json:"name,omitempty"`
	Revision     int32              `json:"revision,omitempty"`
	Namespace    string             `json:"namespace,omitempty"`
	Status       string             `json:"status,omitempty"`
	ChartName    string             `json:"chartName,omitempty"`
	ChartVersion string             `json:"chartVersion,omitempty"`
	Manifest     string             `json:"-"`
	Hooks        []*ReleaseHook     `json:"hooks,omitempty"`
	Resources    []*ReleaseResource `json:"resources,omitempty"`
	Config       string             `json:"config,omitempty"`
	Commit       string             `json:"commit,omitempty"`
}

type ReleaseResource struct {
	Group           string `json:"group,omitempty"`
	Version         string `json:"version,omitempty"`
	Kind            string `json:"kind,omitempty"`
	Name            string `json:"name,omitempty"`
	ResourceVersion string `json:"resourceVersion,omitempty"`
	Object          string `json:"object,omitempty"`
}

type ReleaseHook struct {
	Name        string `json:"name,omitempty"`
	Kind        string `json:"kind,omitempty"`
	Manifest    string `json:"manifest,omitempty"`
	Weight      int32  `json:"weight,omitempty"`
	ReleaseName string `json:"releaseName,omitempty"`
}

type UpgradeReleaseRequest struct {
	ReleaseName      string                         `json:"releaseName,omitempty"`
	RepoURL          string                         `json:"repoURL,omitempty,omitempty"`
	ChartName        string                         `json:"chartName,omitempty"`
	ChartVersion     string                         `json:"chartVersion,omitempty"`
	Values           string                         `json:"values,omitempty"`
	Command          int                            `json:"command,omitempty"`
	Commit           string                         `json:"commit,omitempty"`
	Namespace        string                         `json:"namespace,omitempty"`
	ImagePullSecrets []core_v1.LocalObjectReference `json:"imagePullSecrets,omitempty"`
}

type RollbackReleaseRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Version     int    `json:"version,omitempty"`
}

type DeleteReleaseRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type StopReleaseRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

type StopReleaseResponse struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type StartReleaseRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
}

type StartReleaseResponse struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type GetReleaseContentRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Version     int32  `json:"version,omitempty"`
}

type SyncRequest struct {
	ResourceType   string `json:"resourceType,omitempty"`
	ResourceName   string `json:"resourceName,omitempty"`
	Commit         string `json:"commit,omitempty"`
	Id             int32  `json:"id,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
	ResourceStatus string `json:"resourceStatus,omitempty"`
}

type CertManagerInfo struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Namespace   string `json:"namespace,omitempty"`
	Version     string `json:"version,omitempty"`
}


