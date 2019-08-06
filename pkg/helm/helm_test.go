package helm

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"os"
	"testing"
)

func TestGetCertManagerIssuerData(t *testing.T) {
	if getCertManagerIssuerData() != `apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: localhost
spec:
  acme:
    server: https://acme-staging.api.letsencrypt.org/directory
    email: change_it@choerodon.io 
    privateKeySecretRef:
      name: localhost
    http01: {}
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: change_it@choerodon.io 
    privateKeySecretRef:
      name: letsencrypt-prod
    http01: {}` {
		t.Fatal("format 01 cluster issuers incorrect")
	}

	err := os.Setenv("ACME_EMAIL", "test@c7n.co")
	if err == nil && getCertManagerIssuerData() != `apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: localhost
spec:
  acme:
    server: https://acme-staging.api.letsencrypt.org/directory
    email: test@c7n.co 
    privateKeySecretRef:
      name: localhost
    http01: {}
---
apiVersion: certmanager.k8s.io/v1alpha1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: test@c7n.co 
    privateKeySecretRef:
      name: letsencrypt-prod
    http01: {}` {
		t.Fatal("format 02 cluster issuers incorrect")
	}
	os.Unsetenv("ACME_EMAIL")
}

type helmClientTest struct {
	mock.Mock
}

func (c *helmClientTest) ListRelease(namespace string) ([]*Release, error) {
	args := c.Called(namespace)
	return args.Get(0).([]*Release), args.Error(0)
}

func (c *helmClientTest) ExecuteTest(request *TestReleaseRequest) (*TestReleaseResponse, error) {
	args := c.Called(request)
	return args.Get(0).(*TestReleaseResponse), args.Error(1)
}

func (c *helmClientTest) InstallRelease(request *InstallReleaseRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) PreInstallRelease(request *InstallReleaseRequest) ([]*ReleaseHook, error) {
	args := c.Called(request)
	return args.Get(0).([]*ReleaseHook), args.Error(1)
}

func (c *helmClientTest) PreUpgradeRelease(request *UpgradeReleaseRequest) ([]*ReleaseHook, error) {
	args := c.Called(request)
	return args.Get(0).([]*ReleaseHook), args.Error(1)
}

func (c *helmClientTest) UpgradeRelease(request *UpgradeReleaseRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) RollbackRelease(request *RollbackReleaseRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) DeleteRelease(request *DeleteReleaseRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) StartRelease(request *StartReleaseRequest) (*StartReleaseResponse, error) {
	args := c.Called(request)
	return args.Get(0).(*StartReleaseResponse), args.Error(1)
}

func (c *helmClientTest) StopRelease(request *StopReleaseRequest) (*StopReleaseResponse, error) {
	args := c.Called(request)
	return args.Get(0).(*StopReleaseResponse), args.Error(1)
}

func (c *helmClientTest) GetReleaseContent(request *GetReleaseContentRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) GetRelease(request *GetReleaseContentRequest) (*Release, error) {
	args := c.Called(request)
	return args.Get(0).(*Release), args.Error(1)
}

func (c *helmClientTest) DeleteNamespaceReleases(namespaces string) error {
	return nil
}

func TestPreInstallHelmRelease(t *testing.T) {
	helmClient := &helmClientTest{}

	req := &InstallReleaseRequest{
		ChartName:    "test",
		ChartVersion: "0.1.0",
		ReleaseName:  "test",
	}
	releaseHooks := []*ReleaseHook{
		{
			Name:   "name",
			Kind:   "job",
			Weight: 1,
		},
	}

	releaseHooksB, _ := json.Marshal(releaseHooks)
	helmClient.On("PreInstallRelease", req).Return(releaseHooks, nil)

	reqB, _ := json.Marshal(req)
	cmd := &model.Packet{
		Type:    model.HelmInstallJobInfo,
		Payload: string(reqB),
	}

	opts := &command.Opts{
		GitConfig:  git.Config{},
		HelmClient: helmClient,
	}
	newCmds, resp := helm.InstallJobInfo(opts, cmd)

	assert.Equal(t, len(newCmds), 1, "only one new command")
	assert.Equal(t, model.HelmReleaseInstallResourceInfo, newCmds[0].Type, "get install command")
	assert.Equal(t, string(reqB), newCmds[0].Payload, "equal request")
	assert.Equal(t, model.HelmInstallJobInfo, resp.Type, "pre install response")
	assert.Equal(t, string(releaseHooksB), resp.Payload, "payload is hook list")
}
