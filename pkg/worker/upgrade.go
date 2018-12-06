package worker

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
)

var choerodonId string

func upgrade(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	upgradeInfo,certInfo,err := w.helmClient.ListAgent(cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.UpgradeClusterFailed, err)
	}
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

	if certInfo != nil{
		certRsp, err := json.Marshal(upgradeInfo)
		if err != nil{
			glog.Errorf("check cert manager error while marshal cert rsp")
		} else {
			certInfoResp := &model.Packet{
				Key:     cmd.Key,
				Type:    model.CertManagerConfig,
				Payload: string(certRsp),
			}
			w.chans.ResponseChan <- certInfoResp
		}

	} else {
		certInfoResp := &model.Packet{
			Key:     cmd.Key,
			Type:    model.CertManagerConfig,
		}
		w.chans.ResponseChan <- certInfoResp
	}
	return nil, resp
}
