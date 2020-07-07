package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const SecretName = "chart-authentication"

type ChartAccount struct {
	RepoUrl  string `json:"url"`
	Username string `json:"userName"`
	Password string `json:"password"`
}

func AddHelmAccount(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	chartAccount := ChartAccount{}
	err := json.Unmarshal([]byte(cmd.Payload), &chartAccount)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.ChartMuseumAuthenticationFailed, err)
	}

	secret, err := opts.KubeClient.GetKubeClient().CoreV1().Secrets(kube.AgentNamespace).Get(SecretName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			// secret 不存在，创建secret
			secret = &v1.Secret{}
			secret.SetName(SecretName)
			chartAccountJsonContent, err := makechartaccountJsonContent([]byte{}, chartAccount)
			secret.Data["info"] = chartAccountJsonContent

			if secret, err = opts.KubeClient.GetKubeClient().CoreV1().Secrets(kube.AgentNamespace).Create(secret); err != nil {
				return nil, command.NewResponseError(cmd.Key, model.ChartMuseumAuthenticationFailed, err)
			}
			resp, err := getResponsePacket(secret, cmd)
			if err != nil {
				return nil, command.NewResponseError(cmd.Key, model.OperateDockerRegistrySecretFailed, err)
			}
			return nil, resp
		}
		return nil, command.NewResponseError(cmd.Key, model.ChartMuseumAuthenticationFailed, err)
	}
	// secret存在，更新secret
	chartAccountJsonContent, err := makechartaccountJsonContent(secret.Data["info"], chartAccount)

	secret.Data["info"] = chartAccountJsonContent

	secret, err = opts.KubeClient.GetKubeClient().CoreV1().Secrets(kube.AgentNamespace).Update(secret)

	resp, err := getResponsePacket(secret, cmd)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.ChartMuseumAuthenticationFailed, err)
	}
	return nil, resp
}

func makechartaccountJsonContent(info []byte, chartAccount ChartAccount) ([]byte, error) {
	chartAccountMap := make(map[string]ChartAccount)
	if err := json.Unmarshal(info, &chartAccountMap); err != nil {
		return nil, err
	}
	chartAccountMap[chartAccount.RepoUrl] = chartAccount

	var chartAccountJsonContent []byte
	chartAccountJsonContent, err := json.Marshal(chartAccountMap)
	if err != nil {
		return nil, err
	}
	return chartAccountJsonContent, nil
}

func getResponsePacket(secret *v1.Secret, cmd *model.Packet) (*model.Packet, error) {
	secretStr, err := json.Marshal(secret)
	if err != nil {
		return nil, err
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.OperateDockerRegistrySecret,
		Payload: string(secretStr),
	}
	return resp, nil
}
