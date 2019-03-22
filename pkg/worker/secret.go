package worker

import (
	"encoding/base64"
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	core_v1 "k8s.io/api/core/v1"
)

func init() {
	registerCmdFunc(model.OperateDockerRegistrySecret, createDockerRegistrySecret)
}

func createDockerRegistrySecret(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	dockerCfg := &DockerConfigEntry{}
	err := json.Unmarshal([]byte(cmd.Payload), dockerCfg)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperateDockerRegistrySecretFailed, err)
	}
	raw := &core_v1.Secret{}
	raw.SetName(dockerCfg.Name)
	dockerCfgJSONContent, err := handleDockerCfgJSONContent(
		dockerCfg.Username,
		dockerCfg.Password,
		dockerCfg.Email,
		dockerCfg.Server,
	)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperateDockerRegistrySecretFailed, err)
	}
	raw.Data = make(map[string][]byte, 0)
	raw.Data[core_v1.DockerConfigJsonKey] = dockerCfgJSONContent
	raw.Type = core_v1.SecretTypeDockerConfigJson
	secret, err := w.kubeClient.CreateOrUpdateDockerRegistrySecret(dockerCfg.Namespace, raw)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperateDockerRegistrySecretFailed, err)
	}
	secretStr, err := json.Marshal(secret)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperateDockerRegistrySecretFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.OperateDockerRegistrySecret,
		Payload: string(secretStr),
	}
	return nil, resp
}

// handleDockerCfgJSONContent serializes a ~/.docker/config.json file
func handleDockerCfgJSONContent(username, password, email, server string) ([]byte, error) {
	dockercfgAuth := DockerConfigEntry{
		Username: username,
		Password: password,
		Email:    email,
		Auth:     encodeDockerConfigFieldAuth(username, password),
	}

	dockerCfgJSON := DockerConfigJSON{
		Auths: map[string]DockerConfigEntry{server: dockercfgAuth},
	}

	return json.Marshal(dockerCfgJSON)
}

func encodeDockerConfigFieldAuth(username, password string) string {
	fieldValue := username + ":" + password
	return base64.StdEncoding.EncodeToString([]byte(fieldValue))
}

// DockerConfigJSON represents a local docker auth config file
// for pulling images.
type DockerConfigJSON struct {
	Auths DockerConfig `json:"auths"`
	// +optional
	HttpHeaders map[string]string `json:"HttpHeaders,omitempty"`
}

// DockerConfig represents the config file used by the docker CLI.
// This config that represents the credentials that should be used
// when pulling images from specific image repositories.
type DockerConfig map[string]DockerConfigEntry

type DockerConfigEntry struct {
	Name      string `json:"name,omitempty"`
	Auth      string `json:"auth,omitempty"`
	Email     string `json:"email,omitempty"`
	Server    string `json:"server,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Username  string `json:"username,omitempty"`
	Password  string `json:"password,omitempty"`
}
