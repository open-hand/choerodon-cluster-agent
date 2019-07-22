package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/scale/scheme"
	"k8s.io/kubernetes/pkg/kubectl"
)

func ScalePod(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *kubernetes.ScalePodRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	clientSet := opts.KubeClient.GetKubeClient()
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	scaleClient := scale.New(clientSet.RESTClient(), nil, nil, nil)

	scaler := kubectl.NewScaler(scaleClient)

	gr := scheme.Resource("deployment")

	precondition := &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}
	_, err = scaler.ScaleSimple(req.Namespace, req.DeploymentName, precondition, uint(req.Count), gr)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}
	resp := &model.Packet{
		Key:  cmd.Key,
		Type: model.OperatePodCountSuccess,
	}
	return nil, resp
}
