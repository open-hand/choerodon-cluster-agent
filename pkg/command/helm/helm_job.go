package helm

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"strings"
	"time"
)

////helm 安装
func InstallJobInfo(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.InstallReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}

	//如果是prometheus 那么就不进行下面这些花里胡哨的步骤，直接安装
	if req.ChartName == "prometheus-operator" {
		installPrometheusCmd := &model.Packet{
			Key:     cmd.Key,
			Type:    model.HelmReleaseInstallResourceInfo,
			Payload: cmd.Payload,
		}
		newCmds = append(newCmds, installPrometheusCmd)
		return newCmds, nil
	}

	username, password, err := GetCharUsernameAndPassword(opts, cmd)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	// 这一步获得的hook信息似乎就只返回给了devops，并没有其他的实质性作用
	hooks, err := opts.HelmClient.PreInstallRelease(&req, username, password)
	if err != nil {
		// 如果是EOF错误，则是chart包下载或者读取问题，再重新执行安装操作，如果失败次数达到5次，则安装失败
		if strings.Contains(err.Error(), "EOF") {
			glog.Errorf("type:%s release:%s  err: EOF,try to reinstall", model.HelmInstallJobInfo, req.ReleaseName)
			if req.FailedCount == 5 {
				glog.Infof("type:%s release:%s install failed With 5 times retry", model.HelmInstallJobInfo, req.ReleaseName)
				return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, fmt.Errorf("install failed With 5 times retry,err:%v", req.LasttimeFailedInstallErr))
			} else {
				req.FailedCount++
				req.LasttimeFailedInstallErr = err.Error()
				reqBytes, err := json.Marshal(req)
				if err != nil {
					return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
				}
				packet := &model.Packet{
					Key:     fmt.Sprintf("env:%s.release:%s", req.Namespace, req.ReleaseName),
					Type:    model.HelmInstallJobInfo,
					Payload: string(reqBytes),
				}
				// 等待10s
				time.Sleep(10 * time.Second)
				glog.Infof("type:%s release:%s  resend install command", model.HelmInstallJobInfo, req.ReleaseName)
				opts.CrChan.CommandChan <- packet
				return nil, nil
			}
		}
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmInstallJobInfo,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseInstallResourceInfo,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}

func UpgradeJobInfo(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
	//如果是prometheus 那么就不进行下面这些花里胡哨的步骤，直接安装
	if req.ChartName == "prometheus-operator" {
		installPrometheusCmd := &model.Packet{
			Key:     cmd.Key,
			Type:    model.HelmReleaseUpgradeResourceInfo,
			Payload: cmd.Payload,
		}
		newCmds = append(newCmds, installPrometheusCmd)
		return newCmds, nil
	}
	username, password, err := GetCharUsernameAndPassword(opts, cmd)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooks, err := opts.HelmClient.PreUpgradeRelease(&req, username, password)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmUpgradeJobInfo,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgradeResourceInfo,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}

func CrossUpgradeJobInfo(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	deleteReq := helm.DeleteReleaseRequest{
		ReleaseName: req.ReleaseName,
		Namespace:   req.Namespace,
	}
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseCrossUpgradeFailed, err)
	}
	_, err = opts.HelmClient.DeleteRelease(&deleteReq)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseCrossUpgradeFailed, err)
	}
	glog.Info("Old instance %s was deleted successfully", req.ReleaseName)

	installCmd := &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", req.Namespace, req.ReleaseName),
		Type:    model.HelmInstallJobInfo,
		Payload: cmd.Payload,
	}

	newCmds = append(newCmds, installCmd)

	return newCmds, nil
}
