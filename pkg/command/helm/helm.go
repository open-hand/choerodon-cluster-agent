package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
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
	opts.HelmClient.InstallRelease(&req)
	return nil,nil
	//resp, err :=
	//if err != nil {
	//	return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	//}
	//respB, err := json.Marshal(resp)
	//if err != nil {
	//	return nil, command.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	//}
	//return nil, &model.Packet{
	//	Key:     cmd.Key,
	//	Type:    model.HelmReleaseInstallResourceInfo,
	//	Payload: string(respB),
	//}

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
	return DeleteHelmRelease(opts, cmd)
}

