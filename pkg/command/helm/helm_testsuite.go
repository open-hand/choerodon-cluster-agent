package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"strings"
)

func ExecuteTestRelease(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.TestReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	req.Label = opts.PlatformCode
	resp, err := opts.HelmClient.ExecuteTest(&req)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.ExecuteTestFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.ExecuteTestSucceed,
		Payload: string(respB),
	}
}

func GetTestStatus(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	releaseNames := make([]string, 0)
	err := json.Unmarshal([]byte(cmd.Payload), &releaseNames)
	if err != nil {
		glog.Errorf("unmarshal test status request error %v,", err)
		return nil, nil
	}

	releasesStatus := make([]helm.TestReleaseStatus, 0)
	for _, rls := range releaseNames {
		status := releaseStatus(opts, rls)
		if status != "" {
			testRlsStatus := helm.TestReleaseStatus{
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

func releaseStatus(opts *command.Opts, releaseName string) string {
	_, err := opts.HelmClient.GetRelease(&helm.GetReleaseContentRequest{ReleaseName: releaseName})
	if err != nil {
		if strings.Contains(err.Error(), "not exist") {
			return "delete"
		}
		return ""
	}
	jobRun := opts.KubeClient.IsReleaseJobRun("choerodon-test", releaseName)
	if jobRun {
		return "running"
	} else {
		return "finished"
	}
}
