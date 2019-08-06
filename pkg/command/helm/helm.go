package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

func CertManagerInstall(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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

func RollbackHelmRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.RollbackReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	resp, err := opts.HelmClient.RollbackRelease(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseRollback,
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
