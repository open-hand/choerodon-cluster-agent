package helm

type InstallReleaseRequest struct {
	RepoURL      string `json:"repoURL,omitempty"`
	ChartName    string `json:"chartName,omitempty"`
	ChartVersion string `json:"chartVersion,omitempty"`
	Values       string `json:"values,omitempty"`
	ReleaseName  string `json:"releaseName,omitempty"`
	Commit       string `json:"commit,omitempty"`
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
	Commit       string				`json:"commit,omitempty"`
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
	ReleaseName  string `json:"releaseName,omitempty"`
	RepoURL      string `json:"repoURL,omitempty,omitempty"`
	ChartName    string `json:"chartName,omitempty"`
	ChartVersion string `json:"chartVersion,omitempty"`
	Values       string `json:"values,omitempty"`
	Commit       string	`json:"commit,omitempty"`
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
}

type StopReleaseResponse struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type StartReleaseRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type StartReleaseResponse struct {
	ReleaseName string `json:"releaseName,omitempty"`
}

type GetReleaseContentRequest struct {
	ReleaseName string `json:"releaseName,omitempty"`
	Version     int32  `json:"version,omitempty"`
}
