package helm

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"strings"

	"github.com/golang/glog"
	"github.com/spf13/pflag"
	"k8s.io/helm/pkg/chartutil"
	"k8s.io/helm/pkg/helm"
	"k8s.io/helm/pkg/helm/environment"
	"k8s.io/helm/pkg/hooks"
	"k8s.io/helm/pkg/proto/hapi/chart"
	"k8s.io/helm/pkg/proto/hapi/release"
	"k8s.io/helm/pkg/tiller"
	tillerenv "k8s.io/helm/pkg/tiller/environment"
	"k8s.io/helm/pkg/timeconv"

	envkube "github.com/choerodon/choerodon-agent/pkg/kube"
	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

const (
	notesFileSuffix = "NOTES.txt"
)

var (
	settings environment.EnvSettings
	// ErrReleaseNotFound indicates that a release is not found.
	ErrReleaseNotFound = func(release string) error { return fmt.Errorf("release: %q not found", release) }
	deletePolices      = map[string]release.Hook_DeletePolicy{
		hooks.HookSucceeded: release.Hook_SUCCEEDED,
		hooks.HookFailed:    release.Hook_FAILED,
	}
	events = map[string]release.Hook_Event{
		hooks.PreInstall:         release.Hook_PRE_INSTALL,
		hooks.PostInstall:        release.Hook_POST_INSTALL,
		hooks.PreDelete:          release.Hook_PRE_DELETE,
		hooks.PostDelete:         release.Hook_POST_DELETE,
		hooks.PreUpgrade:         release.Hook_PRE_UPGRADE,
		hooks.PostUpgrade:        release.Hook_POST_UPGRADE,
		hooks.PreRollback:        release.Hook_PRE_ROLLBACK,
		hooks.PostRollback:       release.Hook_POST_ROLLBACK,
		hooks.ReleaseTestSuccess: release.Hook_RELEASE_TEST_SUCCESS,
		hooks.ReleaseTestFailure: release.Hook_RELEASE_TEST_FAILURE,
	}
)

type Client interface {
	ListRelease(namespace string) ([]*model_helm.Release, error)
	InstallRelease(request *model_helm.InstallReleaseRequest) (*model_helm.Release, error)
	PreInstallRelease(request *model_helm.InstallReleaseRequest) ([]*model_helm.ReleaseHook, error)
	PreUpgradeRelease(request *model_helm.UpgradeReleaseRequest) ([]*model_helm.ReleaseHook, error)
	UpgradeRelease(request *model_helm.UpgradeReleaseRequest) (*model_helm.Release, error)
	RollbackRelease(request *model_helm.RollbackReleaseRequest) (*model_helm.Release, error)
	DeleteRelease(request *model_helm.DeleteReleaseRequest) (*model_helm.Release, error)
	StartRelease(request *model_helm.StartReleaseRequest) (*model_helm.StartReleaseResponse, error)
	StopRelease(request *model_helm.StopReleaseRequest) (*model_helm.StopReleaseResponse, error)
	GetReleaseContent(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error)
}

type client struct {
	helmClient *helm.Client
	kubeClient envkube.Client
	namespace  string
}

func init() {
	settings.AddFlags(pflag.CommandLine)
}

func NewClient(
	kubeClient envkube.Client,
	namespace string) Client {
	if _, err := os.Stat(settings.Home.Archive()); os.IsNotExist(err) {
		os.MkdirAll(settings.Home.Archive(), 0755)

	}
	if _, err := os.Stat(settings.Home.Repository()); os.IsNotExist(err) {
		os.MkdirAll(settings.Home.Repository(), 0755)
		ioutil.WriteFile(settings.Home.RepositoryFile(),
			[]byte("apiVersion: v1\nrepositories: []"), 0644)
	}

	setupConnection()
	helmClient := helm.NewClient(helm.Host(settings.TillerHost), helm.ConnectTimeout(settings.TillerConnectionTimeout))

	return &client{
		helmClient: helmClient,
		kubeClient: kubeClient,
		namespace:  namespace,
	}
}

func (c *client) ListRelease(namespace string) ([]*model_helm.Release, error) {
	releases := make([]*model_helm.Release, 0)
	hlr, err := c.helmClient.ListReleases(helm.ReleaseListNamespace(namespace))
	if err != nil {
		glog.Errorf("helm client list release error", err)
	}

	for _, hr := range hlr.Releases {
		re := &model_helm.Release{
			Namespace:    namespace,
			Name:         hr.Name,
			Revision:     hr.Version,
			Status:       hr.Info.Status.Code.String(),
			ChartName:    hr.Chart.Metadata.Name,
			ChartVersion: hr.Chart.Metadata.Version,
			Config:       hr.Config.Raw,
		}
		releases = append(releases, re)
	}
	return releases, nil
}

func (c *client) PreInstallRelease(request *model_helm.InstallReleaseRequest) ([]*model_helm.ReleaseHook, error) {
	var releaseHooks []*model_helm.ReleaseHook

	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp != nil {
		return nil, fmt.Errorf("release %s already exist", request.ReleaseName)
	}

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	rlsHooks, _, err := c.renderManifests(
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	for _, hook := range rlsHooks {
		releaseHook := &model_helm.ReleaseHook{
			Name:        hook.Name,
			Manifest:    hook.Manifest,
			Weight:      hook.Weight,
			Kind:        hook.Kind,
			ReleaseName: request.ReleaseName,
		}
		releaseHooks = append(releaseHooks, releaseHook)
	}

	return releaseHooks, nil
}

func (c *client) InstallRelease(request *model_helm.InstallReleaseRequest) (*model_helm.Release, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp != nil {
		return nil, fmt.Errorf("release %s already exist", request.ReleaseName)
	}

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	_, manifestDoc, err := c.renderManifests(
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}
	if manifestDoc != nil {
		newManifestBuf, err := c.kubeClient.LabelObjects(c.namespace, manifestDoc.String(), request.ReleaseName)
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		chartRequested.Templates = []*chart.Template{
			{Name: request.ReleaseName, Data: newManifestBuf.Bytes()},
		}
	}

	installReleaseResp, err := c.helmClient.InstallReleaseFromChart(
		chartRequested,
		c.namespace,
		helm.ValueOverrides([]byte(request.Values)),
		helm.ReleaseName(request.ReleaseName),
	)
	if err != nil {
		newError := fmt.Errorf("install release %s: %v", request.ReleaseName, err)
		if installReleaseResp != nil {
			rls, err := c.getHelmRelease(installReleaseResp.GetRelease())
			if err != nil {
				return nil, err
			}
			return rls, newError
		}
		return nil, newError
	}
	rls, err := c.getHelmRelease(installReleaseResp.GetRelease())
	if err != nil {
		return nil, err
	}
	return rls, err
}

func (c *client) getHelmRelease(release *release.Release) (*model_helm.Release, error) {
	resources, err := c.kubeClient.GetResources(c.namespace, release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	rlsHooks := make([]*model_helm.ReleaseHook, len(release.GetHooks()))
	for i := 0; i < len(rlsHooks); i++ {
		rlsHook := release.GetHooks()[i]
		rlsHooks[i] = &model_helm.ReleaseHook{
			Name: rlsHook.Name,
			Kind: rlsHook.Kind,
		}
	}
	rls := &model_helm.Release{
		Name:         release.Name,
		Revision:     release.Version,
		Namespace:    release.Namespace,
		Status:       release.Info.Status.Code.String(),
		ChartName:    release.Chart.Metadata.Name,
		ChartVersion: release.Chart.Metadata.Version,
		Manifest:     release.Manifest,
		Hooks:        rlsHooks,
		Resources:    resources,
		Config:       release.Config.Raw,
	}
	return rls, nil
}

func (c *client) PreUpgradeRelease(request *model_helm.UpgradeReleaseRequest) ([]*model_helm.ReleaseHook, error) {
	var releaseHooks []*model_helm.ReleaseHook

	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		installReq := &model_helm.InstallReleaseRequest{
			RepoURL:      request.RepoURL,
			ChartName:    request.ChartName,
			ChartVersion: request.ChartVersion,
			Values:       request.Values,
			ReleaseName:  request.ReleaseName,
		}
		return c.PreInstallRelease(installReq)
	}

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	revision := int(releaseContentResp.Release.Version + 1)
	rlsHooks, _, err := c.renderManifests(
		chartRequested,
		request.ReleaseName,
		request.Values,
		revision)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	for _, hook := range rlsHooks {
		releaseHook := &model_helm.ReleaseHook{
			Name:        hook.Name,
			Manifest:    hook.Manifest,
			Weight:      hook.Weight,
			Kind:        hook.Kind,
			ReleaseName: request.ReleaseName,
		}
		releaseHooks = append(releaseHooks, releaseHook)
	}

	return releaseHooks, nil
}

func (c *client) UpgradeRelease(request *model_helm.UpgradeReleaseRequest) (*model_helm.Release, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		installReq := &model_helm.InstallReleaseRequest{
			RepoURL:      request.RepoURL,
			ChartName:    request.ChartName,
			ChartVersion: request.ChartVersion,
			Values:       request.Values,
			ReleaseName:  request.ReleaseName,
		}
		installResp, err := c.InstallRelease(installReq)
		if err != nil {
			return nil, err
		}
		return installResp, nil
	}

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	_, manifestDoc, err := c.renderManifests(
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}
	if manifestDoc != nil {
		newManifestBuf, err := c.kubeClient.LabelObjects(c.namespace, manifestDoc.String(), request.ReleaseName)
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		chartRequested.Templates = []*chart.Template{
			{Name: request.ReleaseName, Data: newManifestBuf.Bytes()},
		}
	}

	updateReleaseResp, err := c.helmClient.UpdateReleaseFromChart(
		request.ReleaseName,
		chartRequested,
		helm.UpdateValueOverrides([]byte(request.Values)),
	)
	if err != nil {
		newErr := fmt.Errorf("update release %s: %v", request.ReleaseName, err)
		if updateReleaseResp != nil {
			rls, err := c.getHelmRelease(updateReleaseResp.GetRelease())
			if err != nil {
				return nil, err
			}
			return rls, newErr
		}
		return nil, newErr
	}

	rls, err := c.getHelmRelease(updateReleaseResp.GetRelease())
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func (c *client) RollbackRelease(request *model_helm.RollbackReleaseRequest) (*model_helm.Release, error) {
	rollbackReleaseResp, err := c.helmClient.RollbackRelease(
		request.ReleaseName,
		helm.RollbackVersion(int32(request.Version)))
	if err != nil {
		return nil, fmt.Errorf("rollback release %s: %v", request.ReleaseName, err)
	}
	rls, err := c.getHelmRelease(rollbackReleaseResp.Release)
	if err != nil {
		return nil, err
	}

	return rls, nil
}

func (c *client) DeleteRelease(request *model_helm.DeleteReleaseRequest) (*model_helm.Release, error) {
	deleteReleaseResp, err := c.helmClient.DeleteRelease(
		request.ReleaseName,
		helm.DeletePurge(true),
	)
	if err != nil {
		return nil, fmt.Errorf("delete release %s: %v", request.ReleaseName, err)
	}
	rls, err := c.getHelmRelease(deleteReleaseResp.Release)
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func (c *client) StopRelease(request *model_helm.StopReleaseRequest) (*model_helm.StopReleaseResponse, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		return nil, fmt.Errorf("release %s not exist", request.ReleaseName)
	}

	err = c.kubeClient.StopResources(c.namespace, releaseContentResp.Release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	resp := &model_helm.StopReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

func (c *client) StartRelease(request *model_helm.StartReleaseRequest) (*model_helm.StartReleaseResponse, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		return nil, fmt.Errorf("release %s not exist", request.ReleaseName)
	}

	err = c.kubeClient.StartResources(c.namespace, releaseContentResp.Release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	resp := &model_helm.StartReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

func (c *client) renderManifests(
	chartRequested *chart.Chart,
	releaseName string,
	values string,
	revision int) ([]*release.Hook, *bytes.Buffer, error) {
	env := tillerenv.New()

	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      releaseName,
		Time:      ts,
		Namespace: c.namespace,
		Revision:  revision,
		IsInstall: true,
	}
	valuesConfig := &chart.Config{Raw: values}
	clientSet, err := c.kubeClient.GetClientSet()
	if err != nil {
		return nil, nil, err
	}
	caps, err := capabilities(clientSet.Discovery())

	valuesToRender, err := chartutil.ToRenderValuesCaps(chartRequested, valuesConfig, options, caps)
	if err != nil {
		return nil, nil, err
	}
	if err != nil {
		glog.V(1).Infof("unmarshal error...")
		return nil, nil, err
	}

	renderer := env.EngineYard.Default()
	if chartRequested.Metadata.Engine != "" {
		if r, ok := env.EngineYard.Get(chartRequested.Metadata.Engine); ok {
			renderer = r
		} else {
			glog.Infof("warning: %s requested non-existent template engine %s", chartRequested.Metadata.Name, chartRequested.Metadata.Engine)
		}
	}
	files, err := renderer.Render(chartRequested, valuesToRender)
	if err != nil {
		glog.V(1).Infof("render error...")
		return nil, nil, err
	}
	for k, _ := range files {
		if strings.HasSuffix(k, notesFileSuffix) {
			delete(files, k)
		}
	}
	hooks, manifests, err := sortManifests(files, caps.APIVersions, tiller.InstallOrder)
	if err != nil {
		b := bytes.NewBuffer(nil)
		for name, content := range files {
			if len(strings.TrimSpace(content)) == 0 {
				continue
			}
			b.WriteString("\n---\n# Source: " + name + "\n")
			b.WriteString(content)
		}
		return nil, b, err
	}

	b := bytes.NewBuffer(nil)
	for _, m := range manifests {
		b.WriteString("\n---\n# Source: " + m.Name + "\n")
		b.WriteString(m.Content)
	}

	return hooks, b, nil
}

func (c *client) GetReleaseContent(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName, helm.ContentReleaseVersion(request.Version))
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		return nil, fmt.Errorf("release %s not exist", request.ReleaseName)
	}
	rls, err := c.getHelmRelease(releaseContentResp.Release)
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func InitEnvSettings() {
	// set defaults from environment
	settings.Init(pflag.CommandLine)
}
