package version

import "encoding/json"

var (
	GitVersion = ""
	// GitCommit is the git sha1
	GitCommit = ""
	// GitTreeState is the state of the git tree
	GitTreeState = ""
)

type Version struct {
	SemVer       string `json:"sem_ver,omitempty"`
	GitCommit    string `json:"git_commit,omitempty"`
	GitTreeState string `json:"git_tree_state,omitempty"`
}

func (v *Version) String() string {
	vB, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err.Error()
	}
	return string(vB)
}

func GetVersion() *Version {
	return &Version{
		SemVer:       GitVersion,
		GitCommit:    GitCommit,
		GitTreeState: GitTreeState,
	}
}
