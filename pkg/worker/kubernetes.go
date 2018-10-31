package worker

import (
	"encoding/json"
	"io"
	"io/ioutil"

	"github.com/choerodon/choerodon-cluster-agent/pkg/common"
	"github.com/choerodon/choerodon-cluster-agent/pkg/controls"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	model_kubernetes "github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
)

func init() {
	registerCmdFunc(model.KubernetesGetLogs, GetLogsByKubernetes)
	registerCmdFunc(model.KubernetesExec, ExecByKubernetes)
}

func GetLogsByKubernetes(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *model_kubernetes.GetLogsByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	readCloser, err := w.kubeClient.GetLogs(req.Namespace, req.PodName, req.ContainerName)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	readWriter := struct {
		io.Reader
		io.Writer
	}{
		readCloser,
		ioutil.Discard,
	}
	pipe, err := controls.NewPipeFromEnds(nil, readWriter, w.appClient, req.PipeID, common.Log)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	pipe.OnClose(func() {
		readCloser.Close()
	})
	return nil, nil
}

func ExecByKubernetes(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *model_kubernetes.ExecByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesExecFailed, err)
	}
	pipe, err := controls.NewPipe(w.appClient, req.PipeID, common.Exec)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesExecFailed, err)
	}
	local, _ := pipe.Ends()
	w.kubeClient.Exec(req.Namespace, req.PodName, req.ContainerName, local)
	return nil, nil
}
