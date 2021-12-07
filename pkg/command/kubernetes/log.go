package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"io"
	"io/ioutil"
)

type GetLogsByKubernetesRequest struct {
	PodName       string `json:"podName,omitempty"`
	ContainerName string `json:"containerName,omitempty"`
	PipeID        string `json:"pipeID,omitempty"`
	Namespace     string `json:"namespace,omitempty"`
	Previous      bool   `json:"previous,omitempty"`
}

func LogsByKubernetes(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *GetLogsByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	readCloser, err := opts.KubeClient.GetLogs(req.Namespace, req.PodName, req.ContainerName, true, false, 500)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	readWriter := struct {
		io.Reader
		io.Writer
	}{
		readCloser,
		ioutil.Discard,
	}
	pipe, err := websocket.NewPipeFromEnds(nil, readWriter, opts.WsClient, req.PipeID, pipeutil.Log, cmd.Key, opts.Token)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	pipe.OnClose(func() {
		readCloser.Close()
	})
	return nil, nil
}

func DownloadLogByKubernetes(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *GetLogsByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesDownloadLogsFailed, err)
	}

	logReadCloser, err := opts.KubeClient.GetLogs(req.Namespace, req.PodName, req.ContainerName, false, req.Previous, -1)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesDownloadLogsFailed, err)
	}

	opts.WsClient.HandleDownloadLog(cmd.Key, opts.Token, logReadCloser)

	return nil, nil
}
