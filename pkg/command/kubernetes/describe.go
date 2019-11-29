package kubernetes

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	ws "github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
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
			return
		}
		describeInfo := opts.Cluster.DescribeResource(req.Namespace, req.Kind, req.Name)
		rawURL := opts.WsClient.URL()
		nowURL := fmt.Sprintf("%s://%s%sdescribe?key=%s", rawURL.Scheme, rawURL.Host, rawURL.Path, cmd.Key)
		conn, _, err := ws.DialWS(nowURL, http.Header{})
		wp := ws.WsPacket{
			Type: "/agent/describe",
			Key:  cmd.Key,
			Data: &model.Packet{
				Key:     cmd.Key,
				Type:    model.ResourceDescribe,
				Payload: describeInfo,
			},
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



