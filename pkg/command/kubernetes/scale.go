package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScalePodRequest struct {
	DeploymentName string `json:"deploymentName,omitempty"`
	Count          int    `json:"count,omitempty"`
	Namespace      string `json:"namespace,omitempty"`
}

func ScalePod(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *ScalePodRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	clientSet := opts.KubeClient.GetKubeClient()
	s, err := clientSet.AppsV1().Deployments(req.Namespace).GetScale(req.DeploymentName, v1.GetOptions{})
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	s.Spec.Replicas = int32(req.Count)

	_, err = clientSet.AppsV1().Deployments(req.Namespace).UpdateScale(req.DeploymentName, s)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	resp := &model.Packet{
		Key:  cmd.Key,
		Type: model.OperatePodCountSuccess,
	}
	return nil, resp
}
