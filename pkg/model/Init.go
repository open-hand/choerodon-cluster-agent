package model

type GitInitConfig struct {
	SshKey string `json:"sshKey,omitempty"`
	GitUrl string `json:"gitUrl,omitempty"`
}

