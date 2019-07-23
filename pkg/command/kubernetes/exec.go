package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
)

type ExecByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
}

func ExecByKubernetes(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *ExecByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesExecFailed, err)
	}
	pipe, err := websocket.NewPipe(opts.WsClient, req.PipeID, pipeutil.Exec)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesExecFailed, err)
	}
	local, _ := pipe.Ends()
	opts.KubeClient.Exec(req.Namespace, req.PodName, req.ContainerName, local)
	return nil, nil
}
