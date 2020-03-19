package upgrade

type EnvSettings struct {
	DryRun           bool
	KubeConfigFile   string
	KubeContext      string
	Label            string
	ReleaseStorage   string
	TillerNamespace  string
	TillerOutCluster bool
}

var (
	settings = EnvSettings{
		ReleaseStorage:   "secrets",
		TillerNamespace:  "kube-system",
		TillerOutCluster: false,
	}
)
