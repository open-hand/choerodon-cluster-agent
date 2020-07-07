package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func GetCharUsernameAndPassword(opts *command.Opts, cmd *model.Packet) (string, string, error) {
	payload := make(map[string]string)
	if err := json.Unmarshal([]byte(cmd.Payload), &payload); err != nil {
		return "", "", err
	}
	repoUrl := payload["repoURL"]

	secret, err := opts.KubeClient.GetKubeClient().CoreV1().Secrets(kube.AgentNamespace).Get(SecretName, metav1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", "", nil
		}
		return "", "", err
	}

	chartAccountMap := make(map[string]ChartAccount)

	if err := json.Unmarshal(secret.Data["info"], &chartAccountMap); err != nil {
		return "", "", err
	}
	if chartAccount, ok := chartAccountMap[repoUrl]; ok {
		return chartAccount.Username, chartAccount.Password, nil
	}

	return "", "", nil
}
