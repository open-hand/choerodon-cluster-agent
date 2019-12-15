package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

//helm 安装
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
		return newCmds,nil
	}

	//这个hooks 是干嘛的呢？
	hooks, err := opts.HelmClient.PreInstallRelease(&req)
	if err != nil {
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
	//这是在干嘛
	hooks, err := opts.HelmClient.PreUpgradeRelease(&req)
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
