package worker

import (
	"encoding/json"

	"github.com/choerodon/choerodon-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

func init() {
	registerCmdFunc(model.HelmInstallRelease, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.installHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleasePreInstall, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.preInstallHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleasePreUpgrade, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.preUpdateHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleaseUpgrade, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.updateHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleaseRollback, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.rollbackHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleaseDelete, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.deleteHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleaseStart, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.startHelmRelease(cmd)
	})
	registerCmdFunc(model.HelmReleaseStop, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.stopHelmRelease(cmd)
	})
}

func (w *workerManager) preInstallHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.InstallReleaseRequest
	var newCmds []*model.Command
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
	}

	hooks, err := w.helmClient.PreInstallRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
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
	return newCmds, resp, true
}

func (w *workerManager) installHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.InstallReleaseRequest
	var installResp *model_helm.InstallReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
	}
	installResp, err = w.helmClient.InstallRelease(&req)
	if err != nil {
		if installResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err)
	} else {
		installRespB, err := json.Marshal(installResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseInstallFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmInstallRelease,
			Payload: string(installRespB),
		}
	}
	return newCmds, resp, true
}

func (w *workerManager) preUpdateHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.UpgradeReleaseRequest
	var newCmds []*model.Command

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
	}
	hooks, err := w.helmClient.PreUpgradeRelease(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
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
	return newCmds, resp, true
}

func (w *workerManager) updateHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.UpgradeReleaseRequest
	var upgradeResp *model_helm.UpgradeReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
	}
	upgradeResp, err = w.helmClient.UpgradeRelease(&req)
	if err != nil {
		if upgradeResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err)
	} else {
		installRespB, err := json.Marshal(upgradeResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseUpgradeFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmReleaseUpgrade,
			Payload: string(installRespB),
		}
	}
	return newCmds, resp, true
}

func (w *workerManager) rollbackHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.RollbackReleaseRequest
	var rollbackResp *model_helm.RollbackReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err), false
	}

	rollbackResp, err = w.helmClient.RollbackRelease(&req)
	if err != nil {
		if rollbackResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err)
	} else {
		upgradeRollbackB, err := json.Marshal(rollbackResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseRollbackFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmReleaseRollback,
			Payload: string(upgradeRollbackB),
		}
	}

	return newCmds, resp, true
}

func (w *workerManager) deleteHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.DeleteReleaseRequest
	var deleteResp *model_helm.DeleteReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err), false
	}

	deleteResp, err = w.helmClient.DeleteRelease(&req)
	if err != nil {
		if deleteResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err)
	} else {
		deleteRespB, err := json.Marshal(deleteResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseDeleteFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmReleaseDelete,
			Payload: string(deleteRespB),
		}
	}

	return newCmds, resp, true
}

func (w *workerManager) stopHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.StopReleaseRequest
	var stopResp *model_helm.StopReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err), false
	}

	stopResp, err = w.helmClient.StopRelease(&req)
	if err != nil {
		if stopResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err)
	} else {
		stopRespB, err := json.Marshal(stopResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseStopFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmReleaseStop,
			Payload: string(stopRespB),
		}
	}

	return newCmds, resp, true
}

func (w *workerManager) startHelmRelease(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	var req model_helm.StartReleaseRequest
	var startResp *model_helm.StartReleaseResponse
	var newCmds []*model.Command
	var resp *model.Response

	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err), false
	}

	startResp, err = w.helmClient.StartRelease(&req)
	if err != nil {
		if startResp == nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err), false
		}
		resp = NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err)
	} else {
		startRespB, err := json.Marshal(startResp)
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.HelmReleaseStartFailed, err), false
		}
		resp = &model.Response{
			Key:     cmd.Key,
			Type:    model.HelmReleaseStart,
			Payload: string(startRespB),
		}
	}

	return newCmds, resp, true
}
