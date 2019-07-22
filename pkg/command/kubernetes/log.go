package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"io"
	"io/ioutil"
)

func LogsByKubernetes(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *kubernetes.GetLogsByKubernetesRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	readCloser, err := opts.KubeClient.GetLogs(req.Namespace, req.PodName, req.ContainerName)
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
	pipe, err := websocket.NewPipeFromEnds(nil, readWriter, opts.WsClient, req.PipeID, pipeutil.Log)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.KubernetesGetLogsFailed, err)
	}
	pipe.OnClose(func() {
		readCloser.Close()
	})
	return nil, nil
}
