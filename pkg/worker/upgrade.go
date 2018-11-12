package worker

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
)

var choerodonId string

func upgrade(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	upgradeInfo,_ := w.helmClient.ListAgent(cmd.Payload)
	upgradeInfo.Token = w.token
	upgradeInfo.PlatformCode = w.platformCode
	rsp, err := json.Marshal(upgradeInfo)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.UpgradeClusterFailed, err)
	}
	glog.Infof("cluster agent upgrade: %s", string(rsp))
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.Upgrade,
		Payload: string(rsp),
	}
	return nil, resp
}
