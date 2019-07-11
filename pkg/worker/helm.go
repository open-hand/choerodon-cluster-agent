package worker

import (
	"encoding/json"
	rand2 "math/rand"
	"time"

	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-cluster-agent/pkg/model/helm"
	"github.com/golang/glog"
	"strings"
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
	registerCmdFunc(model.StatusSync, syncStatus)
	registerCmdFunc(model.ExecuteTest, executeTestRelease)
	registerCmdFunc(model.TestStatusRequest, GetTestStatus)

}

func preInstallHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req model_helm.InstallReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
	hooks, err := w.helmClient.PreInstallRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
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

func installHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req model_helm.InstallReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
	resp, err := w.helmClient.InstallRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmInstallRelease,
		Payload: string(respB),
	}
}

func executeTestRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req model_helm.TestReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	req.Label = w.platformCode
	resp, err := w.helmClient.ExecuteTest(&req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.ExecuteTestSucceed,
		Payload: string(respB),
	}
}

func GetTestStatus(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	//var req model_helm.TestReleaseStatusRequest
	//err := json.Unmarshal([]byte(cmd.Payload), &req)
	//if err != nil {
	//	return nil, NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	//}
	releaseNames := []string{}
	err := json.Unmarshal([]byte(cmd.Payload), &releaseNames)
	if err != nil {
		glog.Errorf("unmarshal test status request error %v,", err)
		return nil, nil
	}

	releasesStatus := []model_helm.TestReleaseStatus{}
	for _, rls := range releaseNames {
		status := releaseStatus(w, rls)
		if status != "" {
			testRlsStatus := model_helm.TestReleaseStatus{
				ReleaseName: rls,
				Status:      status,
			}
			releasesStatus = append(releasesStatus, testRlsStatus)
		}
	}
	contents, err := json.Marshal(releasesStatus)

	if err != nil {
		glog.Errorf("marshal test status request response error %v,", err)
		return nil, nil
	}

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.TestStatusResponse,
		Payload: string(contents),
	}

}

func releaseStatus(w *workerManager, releaseName string) string {
	_, err := w.helmClient.GetRelease(&model_helm.GetReleaseContentRequest{ReleaseName: releaseName})
	if err != nil {
		if strings.Contains(err.Error(), "not exist") {
			return "delete"
		}
		return ""
	}
	jobRun := w.kubeClient.IsReleaseJobRun("choerodon-test", releaseName)
	if jobRun {
		return "running"
	} else {
		return "finished"
	}
}
func preUpdateHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req model_helm.UpgradeReleaseRequest
	var newCmds []*model.Packet
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
	hooks, err := w.helmClient.PreUpgradeRelease(&req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	hooksJsonB, err := json.Marshal(hooks)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
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

func updateHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req model_helm.UpgradeReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}
	resp, err := w.helmClient.UpgradeRelease(&req)
	if err != nil {
		if req.ChartName == "choerodon-cluster-agent" && req.Namespace == "choerodon" {
			go func() {
				randWait := rand2.Intn(20)
				time.Sleep(time.Duration(randWait) * time.Second)
				glog.Infof("start retry upgrade agent ...")
				w.chans.CommandChan <- cmd
			}()
		}
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgrade,
		Payload: string(respB),
	}
}

func rollbackHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseRollback,
		Payload: string(respB),
	}
}

func deleteHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseDelete,
		Payload: string(respB),
	}
}

func stopHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStop,
		Payload: string(respB),
	}
}

func startHelmRelease(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseStart,
		Payload: string(respB),
	}
}

func getHelmReleaseContent(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
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
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseGetContent,
		Payload: string(respB),
	}
}

func syncStatus(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var reqs []model_helm.SyncRequest
	var reps = []*model_helm.SyncRequest{}

	err := json.Unmarshal([]byte(cmd.Payload), &reqs)
	if err != nil {
		glog.Errorf("unmarshal status sync failed %v", err)
		return nil, nil
	}

	for _, syncRequest := range reqs {
		namespace := cmd.Namespace()
		switch syncRequest.ResourceType {
		case "ingress":
			commit, err := w.kubeClient.GetIngress(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "service":
			commit, err := w.kubeClient.GetService(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "certificate":
			commit, err := w.kubeClient.GetSecret(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if commit != "" {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, commit, syncRequest.Id))
			}
			break
		case "instance":
			chr, err := w.kubeClient.GetC7nHelmRelease(namespace, syncRequest.ResourceName)
			if err != nil {
				reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
			} else if chr != nil {
				if chr.Annotations[model.CommitLabel] == syncRequest.Commit {
					release, err := w.helmClient.GetRelease(&model_helm.GetReleaseContentRequest{ReleaseName: syncRequest.ResourceName})
					if err != nil {
						glog.Infof("release %s get error ", syncRequest.ResourceName, err)
						if strings.Contains(err.Error(), "not exist") {
							if w.kubeClient.IsReleaseJobRun(namespace, syncRequest.ResourceName) {
								glog.Errorf("release %s not exist and not job run ", syncRequest.ResourceName, err)
							} else {
								reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
							}
						}
					}
					if release != nil && release.Status == "DEPLOYED" {
						if release.ChartVersion != chr.Spec.ChartVersion || release.Config != chr.Spec.Values {
							glog.Infof("release deployed but not consistent")
							reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, "", syncRequest.Id))
						} else {
							reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, syncRequest.Commit, syncRequest.Id))
						}
					}
				} else {
					reps = append(reps, newSyncResponse(syncRequest.ResourceName, syncRequest.ResourceType, syncRequest.Commit, syncRequest.Id))
				}
			}
			break
		}
	}

	if len(reps) == 0 {
		return nil, nil
	}

	respB, err := json.Marshal(reps)
	if err != nil {
		glog.Errorf("Marshal response error %v", err)
		return nil, nil
	}
	glog.Infof("sync response %s", string(respB))
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.StatusSync,
		Payload: string(respB),
	}
}

func newSyncResponse(name string, reType string, commit string, id int32) *model_helm.SyncRequest {
	return &model_helm.SyncRequest{
		ResourceName: name,
		ResourceType: reType,
		Commit:       commit,
		Id:           id,
	}
}
