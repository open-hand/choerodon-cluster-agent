package polaris

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/validator"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	ws "github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"net/http"
)

type ScanRequestInfo struct {
	RecordId  int    `json:"recordId"`
	Namespace string `json:"namespace"`
}

type Result struct {
	AuditData validator.AuditData    `json:"auditData"`
	Summary   validator.CountSummary `json:"summary"`
}
type ResponseInfo struct {
	RecordId      int    `json:"recordId"`
	PolarisResult Result `json:"polarisResult"`
}

type PolarisPacket struct {
	Type string       `json:"type,omitempty"`
	Key  string       `json:"key,omitempty"`
	Data ResponseInfo `json:"data"`
}

func ScanSystem(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	go func() {
		var req ScanRequestInfo
		err := json.Unmarshal([]byte(cmd.Payload), &req)
		if err != nil {
			glog.Info(err.Error())
			return
		}
		provider, err := kube.CreateResourceProviderFromAPI(opts.KubeClient.GetKubeClient(), "", req.Namespace)
		if err != nil {
			glog.Info(err.Error())
			return
		}
		auditData, err := validator.RunAudit(*opts.PolarisConfig, provider)
		if err != nil {
			glog.Info(err.Error())
			return
		}
		responseInfo := ResponseInfo{RecordId: req.RecordId, PolarisResult: Result{AuditData: auditData, Summary: auditData.GetSummary()}}
		rawURL := opts.WsClient.URL()
		nowURL := fmt.Sprintf("%s://%s%spolaris?%s", rawURL.Scheme, rawURL.Host, rawURL.Path, rawURL.RawQuery)
		conn, _, err := ws.DialWS(nowURL, http.Header{})
		wp := PolarisPacket{
			Type: "polaris",
			Key:  cmd.Key,
			Data: ResponseInfo{
				RecordId:      responseInfo.RecordId,
				PolarisResult: responseInfo.PolarisResult,
			},
		}
		bytes, err := json.Marshal(wp)
		if err != nil {
			if err := conn.WriteMessage(websocket.TextMessage, []byte(err.Error())); err != nil {
				glog.Info(err.Error())
				conn.Close()
			}
		}
		if err := conn.WriteMessage(websocket.TextMessage, bytes); err != nil {
			glog.Info(err.Error())
			conn.Close()
		}
	}()
	return nil, nil
}
