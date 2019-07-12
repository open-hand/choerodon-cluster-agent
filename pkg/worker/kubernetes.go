package worker

import (
	"encoding/json"
	"io"
	"io/ioutil"
	"k8s.io/client-go/scale"
	"k8s.io/client-go/scale/scheme"
	"k8s.io/kubernetes/pkg/kubectl"

	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	model_kubernetes "github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
)

func init() {
	registerCmdFunc(model.KubernetesGetLogs, GetLogsByKubernetes)
	registerCmdFunc(model.KubernetesExec, ExecByKubernetes)
	registerCmdFunc(model.OperatePodCount, ScalePod)
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
	pipe, err := websocket.NewPipeFromEnds(nil, readWriter, w.appClient, req.PipeID, pipeutil.Log)
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
	pipe, err := websocket.NewPipe(w.appClient, req.PipeID, pipeutil.Exec)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.KubernetesExecFailed, err)
	}
	local, _ := pipe.Ends()
	w.kubeClient.Exec(req.Namespace, req.PodName, req.ContainerName, local)
	return nil, nil
}

func ScalePod(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *model_kubernetes.ScalePodRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	clientSet := w.kubeClient.GetKubeClient()
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}

	scaleClient := scale.New(clientSet.RESTClient(), nil, nil, nil)

	scaler := kubectl.NewScaler(scaleClient)

	gr := scheme.Resource("deployment")

	precondition := &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}
	_, err = scaler.ScaleSimple(req.Namespace, req.DeploymentName, precondition, uint(req.Count), gr)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, err)
	}
	resp := &model.Packet{
		Key:  cmd.Key,
		Type: model.OperatePodCountSuccess,
	}
	return nil, resp
}
