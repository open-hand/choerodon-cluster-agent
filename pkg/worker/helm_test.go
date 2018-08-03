package worker

import (
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"

	"time"

	"github.com/choerodon/choerodon-agent/pkg/git"
	"github.com/choerodon/choerodon-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

type helmClientTest struct {
	mock.Mock
}

func (c *helmClientTest) ListRelease(namespace string) ([]*model_helm.Release, error) {
	args := c.Called(namespace)
	return args.Get(0).([]*model_helm.Release), args.Error(0)
}

func (c *helmClientTest) InstallRelease(request *model_helm.InstallReleaseRequest) (*model_helm.Release, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.Release), args.Error(1)
}

func (c *helmClientTest) PreInstallRelease(request *model_helm.InstallReleaseRequest) ([]*model_helm.ReleaseHook, error) {
	args := c.Called(request)
	return args.Get(0).([]*model_helm.ReleaseHook), args.Error(1)
}

func (c *helmClientTest) PreUpgradeRelease(request *model_helm.UpgradeReleaseRequest) ([]*model_helm.ReleaseHook, error) {
	args := c.Called(request)
	return args.Get(0).([]*model_helm.ReleaseHook), args.Error(1)
}

func (c *helmClientTest) UpgradeRelease(request *model_helm.UpgradeReleaseRequest) (*model_helm.Release, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.Release), args.Error(1)
}

func (c *helmClientTest) RollbackRelease(request *model_helm.RollbackReleaseRequest) (*model_helm.Release, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.Release), args.Error(1)
}

func (c *helmClientTest) DeleteRelease(request *model_helm.DeleteReleaseRequest) (*model_helm.Release, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.Release), args.Error(1)
}

func (c *helmClientTest) StartRelease(request *model_helm.StartReleaseRequest) (*model_helm.StartReleaseResponse, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.StartReleaseResponse), args.Error(1)
}

func (c *helmClientTest) StopRelease(request *model_helm.StopReleaseRequest) (*model_helm.StopReleaseResponse, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.StopReleaseResponse), args.Error(1)
}

func (c *helmClientTest) GetReleaseContent(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error) {
	args := c.Called(request)
	return args.Get(0).(*model_helm.Release), args.Error(1)
}

func TestPreInstallHelmRelease(t *testing.T) {
	helmClient := &helmClientTest{}
	wm := NewWorkerManager(
		nil,
		nil,
		nil,
		helmClient,
		nil,
		"",
		git.Config{},
		nil,
		5*time.Minute,
		nil,
		nil,
	)

	req := &model_helm.InstallReleaseRequest{
		ChartName:    "test",
		ChartVersion: "0.1.0",
		ReleaseName:  "test",
	}
	releaseHooks := []*model_helm.ReleaseHook{
		{
			Name:   "name",
			Kind:   "job",
			Weight: 1,
		},
	}
	releaseHooksB, _ := json.Marshal(releaseHooks)
	helmClient.On("PreInstallRelease", req).Return(releaseHooks, nil)

	reqB, _ := json.Marshal(req)
	cmd := &model.Command{
		Type:    model.HelmReleasePreInstall,
		Payload: string(reqB),
	}
	newCmds, resp := preInstallHelmRelease(wm, cmd)

	assert.Equal(t, len(newCmds), 1, "only one new command")
	assert.Equal(t, model.HelmInstallRelease, newCmds[0].Type, "get install command")
	assert.Equal(t, string(reqB), newCmds[0].Payload, "equal request")
	assert.Equal(t, model.HelmReleasePreInstall, resp.Type, "pre install response")
	assert.Equal(t, string(releaseHooksB), resp.Payload, "payload is hook list")
}
