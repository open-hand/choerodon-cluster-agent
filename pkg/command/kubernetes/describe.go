package kubernetes

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	ws "github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"net/http"
)

type DescribeRequest struct {
	DescribeID string `json:"describeID,omitempty"`
	Namespace  string `json:"namespace,omitempty"`
	Kind       string `json:"kind,omitempty"`
	Name       string `json:"name,omitempty"`
}

func Describe(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	go func() {
		var req *DescribeRequest
		err := json.Unmarshal([]byte(cmd.Payload), &req)
		if err != nil {
			glog.Error(err)
			return
		}
		describeInfo := opts.Cluster.DescribeResource(req.Namespace, req.Kind, req.Name)
		rawURL := opts.WsClient.URL()
		nowURL := fmt.Sprintf(ws.BaseUrl, rawURL.Scheme, rawURL.Host, cmd.Key, cmd.Key, model.ClusterId, "agent_describe", opts.Token, model.AgentVersion)
		conn, _, err := ws.DialWS(nowURL, http.Header{})
		wp := model.Packet{
			Key:     cmd.Key,
			Type:    model.ResourceDescribe,
			Payload: describeInfo,
		}
		bytes, err := json.Marshal(wp)
		if err != nil {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error())); err != nil {
				conn.Close()
			}
		}
		if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
			conn.Close()
		}
	}()
	return nil, nil
}
