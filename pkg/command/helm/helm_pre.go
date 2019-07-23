package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

func PreInstallHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.InstallReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
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
		Type:    model.HelmReleasePreInstall,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}

func PreUpdateHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
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
		Type:    model.HelmReleasePreUpgrade,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgrade,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}
