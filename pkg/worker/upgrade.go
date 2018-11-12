package worker

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
)

func upgrade(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	upgradeInfo,_ := w.helmClient.ListAgent(cmd.Payload)
	glog.Infof("result %v", upgradeInfo)
	rsp, err := json.Marshal(upgradeInfo)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.UpgradeClusterFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.Upgrade,
		Payload: string(rsp),
	}
	return nil, resp
}
