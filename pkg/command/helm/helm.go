package helm

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm/helm2to3"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"strings"
	"time"
)

func InstallHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.InstallReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}

	resp, err := opts.HelmClient.InstallRelease(&req)
	if err != nil {
		// 如果是EOF错误，则是chart包下载或者读取问题，再重新执行安装操作，如果失败次数达到5次，则安装失败
		if strings.Contains(err.Error(), "EOF") {
			glog.Errorf("type:%s release:%s err:EOF,try to reinstall", model.HelmReleaseInstallResourceInfo, req.ReleaseName)
			if req.FailedCount == 5 {
				glog.Infof("type:%s release:%s  install failed With 5 times retry", model.HelmReleaseInstallResourceInfo, req.ReleaseName)
				return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, fmt.Errorf("install failed With 3 times retry,err:%v", req.LasttimeFailedInstallErr))
			} else {
				req.FailedCount++
				req.LasttimeFailedInstallErr = err.Error()
				reqBytes, err := json.Marshal(req)
				if err != nil {
					return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
				}
				packet := &model.Packet{
					Key:     fmt.Sprintf("env:%s.release:%s", req.Namespace, req.ReleaseName),
					Type:    model.HelmReleaseInstallResourceInfo,
					Payload: string(reqBytes),
				}
				// 等待10s
				time.Sleep(10 * time.Second)
				glog.Infof("type:%s release:%s  resend install command", model.HelmReleaseInstallResourceInfo, req.ReleaseName)
				opts.CrChan.CommandChan <- packet
				return nil, nil
			}
		}
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseInstallResourceInfo,
		Payload: string(respB),
	}

}

func DeleteHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.DeleteReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	deleteResp, err := opts.HelmClient.DeleteRelease(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	respB, err := json.Marshal(deleteResp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseDelete,
		Payload: string(respB),
	}
}

func UpgradeHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}

	resp, err := opts.HelmClient.UpgradeRelease(&req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgradeResourceInfo,
		Payload: string(respB),
	}
}

//专门用于安装cert-mgr
func InstallCertManager(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	// 安装 helm Release 不返回新 cmd
	return InstallHelmRelease(opts, cmd)
}

//专门用于卸载cert-mgr
func DeleteCertManagerRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var delRequest helm.DeleteReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &delRequest)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	rlsContentRequest := &helm.GetReleaseContentRequest{
		ReleaseName: delRequest.ReleaseName,
		Namespace:   delRequest.Namespace,
	}
	_, err = opts.HelmClient.GetRelease(rlsContentRequest)
	if err != nil {
		//不存在说明cert-manager可能是由helm2管理的，尝试升级到helm3
		if strings.Contains(err.Error(), helm.ErrReleaseNotFound) {
			helm2to3.RunConvert(delRequest.ReleaseName)
			helm2to3.RunCleanup(delRequest.ReleaseName)
		} else {
			return nil, command.NewResponseError(cmd.Key, cmd.Type, err)
		}
	}
	// 不关注删除结果，直接返回cert-manager删除信息
	DeleteHelmRelease(opts, cmd)
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.CertManagerStatus,
		Payload: fmt.Sprintf(model.PodStatus, "deleted"),
	}
}

// TODO devops 没有使用此功能，注释掉，等待以后有相关业务再处理
//func RollbackHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
//	var req helm.RollbackReleaseRequest
//	err := json.Unmarshal([]byte(cmd.Payload), &req)
//	if err != nil {
//		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
//	}
//	resp, err := opts.HelmClient.RollbackRelease(&req)
//	if err != nil {
//		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
//	}
//	respB, err := json.Marshal(resp)
//	if err != nil {
//		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
//	}
//	return nil, &model.Packet{
//		Key:     cmd.Key,
//		Type:    model.HelmReleaseRollback,
//		Payload: string(respB),
//	}
//}
