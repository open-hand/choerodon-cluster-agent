package worker

import (
	"encoding/json"

	"github.com/choerodon/choerodon-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

func init() {
	registerCmdFunc(model.HelmInstallRelease, installHelmRelease)
	registerCmdFunc(model.HelmReleasePreInstall, preInstallHelmRelease)
	registerCmdFunc(model.HelmReleasePreUpgrade, preUpdateHelmRelease)
	registerCmdFunc(model.HelmReleaseUpgrade, updateHelmRelease)
	registerCmdFunc(model.HelmReleaseRollback, rollbackHelmRelease)
	registerCmdFunc(model.HelmReleaseDelete, deleteHelmRelease)
	registerCmdFunc(model.HelmReleaseStart, startHelmRelease)
	registerCmdFunc(model.HelmReleaseStop, stopHelmRelease)
	registerCmdFunc(model.HelmReleaseGetContent, getHelmReleaseContent)
}

func preInstallHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.InstallReleaseRequest
	var newCmds []*model.Command
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooks, err := w.helmClient.PreInstallRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleasePreInstall,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Command{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}

func installHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.InstallReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp, err := w.helmClient.InstallRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: string(respB),
	}
}

func preUpdateHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.UpgradeReleaseRequest
	var newCmds []*model.Command
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooks, err := w.helmClient.PreUpgradeRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleasePreUpgrade,
		Payload: string(hooksJsonB),
	}
	newCmd := &model.Command{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgrade,
		Payload: cmd.Payload,
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, resp
}

func updateHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.UpgradeReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	resp, err := w.helmClient.UpgradeRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgrade,
		Payload: string(respB),
	}
}

func rollbackHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.RollbackReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	resp, err := w.helmClient.RollbackRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseRollback,
		Payload: string(respB),
	}
}

func deleteHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.DeleteReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	deleteResp, err := w.helmClient.DeleteRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	respB, err := json.Marshal(deleteResp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseDelete,
		Payload: string(respB),
	}
}

func stopHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.StopReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	resp, err := w.helmClient.StopRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStop,
		Payload: string(respB),
	}
}

func startHelmRelease(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.StartReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	startResp, err := w.helmClient.StartRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	respB, err := json.Marshal(startResp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStart,
		Payload: string(respB),
	}
}

func getHelmReleaseContent(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model_helm.GetReleaseContentRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	resp, err := w.helmClient.GetReleaseContent(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseGetContentFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.HelmReleaseGetContent,
		Payload: string(respB),
	}
}
