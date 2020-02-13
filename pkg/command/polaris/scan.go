package polaris

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/validator"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	ws "github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"github.com/gorilla/websocket"
	"net/http"
)

type ScanRequestInfo struct {
	Namespace string `json:"env,string"`
}

func ScanSystem(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	go func() {
		var req ScanRequestInfo
		err := json.Unmarshal([]byte(cmd.Payload), &req)
		if err != nil {
			//return nil, command.NewResponseError(cmd.Key, model.PolarisRequestFailed, err)
			return
		}
		provider, err := kube.CreateResourceProviderFromAPI(opts.KubeClient.GetKubeClient(), "", req.Namespace)
		if err != nil {
			//return nil, command.NewResponseError(cmd.Key, model.PolarisRequestFailed, err)
			return
		}
		auditData, err := validator.RunAudit(*opts.PolarisConfig, provider)
		if err != nil {
			//return nil, command.NewResponseError(cmd.Key, model.PolarisRequestFailed, err)
			return
		}
		responseInfo := validator.ResponseInfo{AuditData: auditData, Summary: auditData.GetSummary()}
		result, err := json.Marshal(responseInfo)
		if err != nil {
			//return nil, command.NewResponseError(cmd.Key, model.PolarisRequestFailed, err)
			return
		}
		rawURL := opts.WsClient.URL()
		nowURL := fmt.Sprintf("%s://%s%spolaris?key=%s", rawURL.Scheme, rawURL.Host, rawURL.Path, cmd.Key)
		conn, _, err := ws.DialWS(nowURL, http.Header{})
		wp := ws.WsPacket{
			Type: "/agent/polaris",
			Key:  cmd.Key,
			Data: &model.Packet{
				Key:     cmd.Key,
				Type:    model.PolarisResponse,
				Payload: string(result),
			},
		}
		bytes, err := json.Marshal(wp)
		if err != nil {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error())); err != nil {
				conn.Close()
			}
		}
		if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
			_ = conn.Close()
		}
	}()
	return nil, nil
}
