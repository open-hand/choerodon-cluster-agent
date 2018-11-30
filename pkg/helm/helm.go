package helm

import (
	"bytes"
	"errors"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"io/ioutil"
	"os"
	"regexp"
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

	envkube "github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	model_helm "github.com/choerodon/choerodon-cluster-agent/pkg/model/helm"
	"gopkg.in/yaml.v2"
	"strconv"
)

const (
	notesFileSuffix = "NOTES.txt"
	testNamespace   = "choerodon-test"
	tmpMagicStr = 	"crockitwoodzZ: heihei"
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
	ExecuteTest(request *model_helm.TestReleaseRequest) (*model_helm.TestReleaseResponse, error)
	InstallRelease(request *model_helm.InstallReleaseRequest) (*model_helm.Release, error)
	PreInstallRelease(request *model_helm.InstallReleaseRequest) ([]*model_helm.ReleaseHook, error)
	PreUpgradeRelease(request *model_helm.UpgradeReleaseRequest) ([]*model_helm.ReleaseHook, error)
	UpgradeRelease(request *model_helm.UpgradeReleaseRequest) (*model_helm.Release, error)
	RollbackRelease(request *model_helm.RollbackReleaseRequest) (*model_helm.Release, error)
	DeleteRelease(request *model_helm.DeleteReleaseRequest) (*model_helm.Release, error)
	StartRelease(request *model_helm.StartReleaseRequest) (*model_helm.StartReleaseResponse, error)
	StopRelease(request *model_helm.StopReleaseRequest) (*model_helm.StopReleaseResponse, error)
	GetReleaseContent(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error)
	GetRelease(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error)
    ListAgent(devConnectUrl string) (*model.UpgradeInfo, error)
	DeleteNamespaceReleases(namespaces string) error
}

type client struct {
	helmClient *helm.Client
	kubeClient envkube.Client
}

func init() {
	settings.AddFlags(pflag.CommandLine)
}

func NewClient(
	kubeClient envkube.Client) Client {
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
		request.Namespace,
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


	chartutil.ProcessRequirementsEnabled(chartRequested,&chart.Config{Raw: request.Values})

	cm, err := cmForChart(chartRequested)
	if  err != nil{
		return nil, fmt.Errorf("install chart error: %v", err)
	}
	valuesMap := make(map[string]string)
	valuesMap = getValuesMap(cm, valuesMap)
	oldValues := chartRequested.Values.Raw

	err,newChartValues := removeStringValues(chartRequested.Values.Raw)
	if err != nil {
		return nil, err
	}
	err,newValues := removeStringValues(request.Values)
	if err != nil {
		return nil, err
	}
	chartRequested.Values.Raw = newChartValues

	hooks, manifestDoc, err := c.renderManifests(
		request.Namespace,
		chartRequested,
		request.ReleaseName,
		newValues,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	manifestDocs := []string{}
	newTemplates := []*chart.Template{}

	if manifestDoc != nil {
		manifestDocs = append(manifestDocs, manifestDoc.String())
	}
	for _,hook := range hooks {
		manifestDocs = append(manifestDocs, hook.Manifest)
	}

	for index,manifestToInsert := range manifestDocs  {
		newManifestBuf, err := c.kubeClient.LabelObjects(request.Namespace, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion)
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		manifestBytes :=  []byte(replaceValue(string(newManifestBuf.Bytes()), valuesMap))
		fmt.Println(string(manifestBytes))
		if index == 0 {
			newTemplate := &chart.Template{Name: request.ReleaseName, Data: manifestBytes}
			newTemplates = append(newTemplates, newTemplate)
		} else {
			newTemplate := &chart.Template{Name: "hook"+strconv.Itoa(index), Data: manifestBytes}
			newTemplates = append(newTemplates, newTemplate)
		}
	}


	chartRequested.Templates = newTemplates
	chartRequested.Dependencies = []*chart.Chart{}
	chartRequested.Values.Raw = oldValues
	installReleaseResp, err := c.helmClient.InstallReleaseFromChart(
		chartRequested,
		request.Namespace,
		helm.ValueOverrides([]byte(request.Values)),
		helm.ReleaseName(request.ReleaseName),
	)
	if err != nil {
		newError := fmt.Errorf("install release %s: %v", request.ReleaseName, err)
		if installReleaseResp != nil {
			rls, err := c.getHelmRelease(installReleaseResp.GetRelease())
			if err != nil {
				c.DeleteRelease(&model_helm.DeleteReleaseRequest{ReleaseName: request.ReleaseName})
				return nil, err
			}
			return rls, newError
		}
		c.DeleteRelease(&model_helm.DeleteReleaseRequest{ReleaseName: request.ReleaseName})
		return nil, newError
	}
	rls, err := c.getHelmRelease(installReleaseResp.GetRelease())
	rls.Commit = request.Commit
	if err != nil {
		return nil, err
	}
	return rls, err
}



func (c *client) ExecuteTest(request *model_helm.TestReleaseRequest) (*model_helm.TestReleaseResponse, error) {

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	chartutil.ProcessRequirementsEnabled(chartRequested,&chart.Config{Raw: request.Values})

	hooks, manifestDoc, err := c.renderManifests(
		testNamespace,
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	manifestDocs := []string{}
	newTemplates := []*chart.Template{}

	if manifestDoc != nil {
		manifestDocs = append(manifestDocs, manifestDoc.String())
	}
	for _,hook := range hooks {
		manifestDocs = append(manifestDocs, hook.Manifest)
	}

	for index,manifestToInsert := range manifestDocs  {
		newManifestBuf, err := c.kubeClient.LabelTestObjects(testNamespace, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion, "test-test")
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		if index == 0 {
			newTemplate := &chart.Template{Name: request.ReleaseName, Data: newManifestBuf.Bytes()}
			newTemplates = append(newTemplates, newTemplate)
		} else {
			newTemplate := &chart.Template{Name: "hook"+strconv.Itoa(index), Data: newManifestBuf.Bytes()}
			newTemplates = append(newTemplates, newTemplate)
		}
	}

	chartRequested.Templates = newTemplates
	chartRequested.Dependencies = []*chart.Chart{}
	installReleaseResp, err := c.helmClient.InstallReleaseFromChart(
		chartRequested,
		testNamespace,
		helm.ValueOverrides([]byte(request.Values)),
		helm.ReleaseName(request.ReleaseName),
	)

	resp := &model_helm.TestReleaseResponse{ReleaseName: request.ReleaseName}
	if err != nil {
		newError := fmt.Errorf("execute test release %s: %v", request.ReleaseName, err)
		if installReleaseResp != nil {
			_, err := c.getHelmRelease(installReleaseResp.GetRelease())
			if err != nil {
				c.DeleteRelease(&model_helm.DeleteReleaseRequest{ReleaseName: request.ReleaseName})
				return nil, err
			}

			return resp, newError
		}
		c.DeleteRelease(&model_helm.DeleteReleaseRequest{ReleaseName: request.ReleaseName})
		return nil, newError
	}
	_, err = c.getHelmRelease(installReleaseResp.GetRelease())
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *client) getHelmRelease(release *release.Release) (*model_helm.Release, error) {
	resources, err := c.kubeClient.GetResources(release.Namespace, release.Manifest)
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

func (c *client) getHelmReleaseNoResource(release *release.Release) (*model_helm.Release, error) {
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
			Namespace:    request.Namespace,
		}
		return c.PreInstallRelease(installReq)
	}

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	revision := int(releaseContentResp.Release.Version + 1)
	rlsHooks, _, err := c.renderManifests(
		request.Namespace,
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
			Namespace:    request.Namespace,
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

	chartutil.ProcessRequirementsEnabled(chartRequested,&chart.Config{Raw: request.Values})



	hooks, manifestDoc, err := c.renderManifests(
		request.Namespace,
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	manifestDocs := []string{}
	newTemplates := []*chart.Template{}

	if manifestDoc != nil {
		manifestDocs = append(manifestDocs, manifestDoc.String())
	}
	for _,hook := range hooks {
		manifestDocs = append(manifestDocs, hook.Manifest)
	}

	for index,manifestToInsert := range manifestDocs  {
		newManifestBuf, err := c.kubeClient.LabelObjects(request.Namespace, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion)
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		if index == 0 {
			newTemplate := &chart.Template{Name: request.ReleaseName, Data: newManifestBuf.Bytes()}
			newTemplates = append(newTemplates, newTemplate)
		} else {
			newTemplate := &chart.Template{Name: "hook"+ strconv.Itoa(index), Data: newManifestBuf.Bytes()}
			newTemplates = append(newTemplates, newTemplate)
		}
	}

	chartRequested.Templates = newTemplates
	chartRequested.Dependencies = []*chart.Chart{}

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
	rls.Commit = request.Commit
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

	err = c.kubeClient.StopResources(request.Namespace, releaseContentResp.Release.Manifest)
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

	err = c.kubeClient.StartResources(request.Namespace, releaseContentResp.Release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	resp := &model_helm.StartReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

func (c *client) ListAgent(devConnectUrl string) (*model.UpgradeInfo, error) {
	listReleasesRsp, err := c.helmClient.ListReleases(helm.ReleaseListStatuses([]release.Status_Code{}))
	upgradeInfo :=  &model.UpgradeInfo{
		Envs: []model.OldEnv{},
	}
	if err != nil {
		return nil, err
	}
	for _,rls := range listReleasesRsp.Releases{
		if rls.Chart.Metadata.Name == "choerodon-agent" {
			if c.kubeClient.GetNamespace(rls.Namespace) == nil {
				err,connectUrl,envId := getEnvInfo(rls.Config.Raw)
				if	err != nil {
					glog.Infof("rls: %s upgrade error ", rls.Name)
					continue
				}
				if ! strings.Contains(devConnectUrl, connectUrl)  {
					continue
				}
				stopRls :=	&model_helm.StopReleaseRequest{
					ReleaseName: rls.Name,
					Namespace: rls.Namespace,
				}
				rsp,err :=  c.StopRelease(stopRls)
				if err == nil {
					glog.Infof("stop old agent %s succeed", rsp.ReleaseName)
				} else {
					glog.Warningf("stop old agent %s failed: %v", rls.Name, err)
				}

				oldEnv := model.OldEnv{
					EnvId: envId,
					Namespace: rls.Namespace,

				}
				upgradeInfo.Envs = append(upgradeInfo.Envs, oldEnv)
			}

		}

	}

	return upgradeInfo, nil
}

func (c *client) renderManifests(
	namespace string,
	chartRequested *chart.Chart,
	releaseName string,
	values string,
	revision int) ([]*release.Hook, *bytes.Buffer, error) {
	env := tillerenv.New()

	ts := timeconv.Now()
	options := chartutil.ReleaseOptions{
		Name:      releaseName,
		Time:      ts,
		Namespace: namespace,
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

func (c *client) DeleteNamespaceReleases(namespaces string) error {

	rlss,err :=  c.helmClient.ListReleases(helm.ReleaseListNamespace(namespaces))
	if err != nil {
		glog.Errorf("delete ns release failed %v",err )
		return err
	}
	for _,rls := range rlss.Releases {
		c.helmClient.DeleteRelease(rls.Name,helm.DeletePurge(true))
	}
	return nil

}

func (c *client) GetRelease(request *model_helm.GetReleaseContentRequest) (*model_helm.Release, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName, helm.ContentReleaseVersion(request.Version))
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		return nil, fmt.Errorf("release %s not exist", request.ReleaseName)
	}
	rls, err := c.getHelmReleaseNoResource(releaseContentResp.Release)
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func InitEnvSettings() {
	// set defaults from environment
	settings.Init(pflag.CommandLine)
}



func getEnvInfo (values string) (error, string, int) {
	mapValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(values), mapValues); err != nil {
		return fmt.Errorf("unmarshal values err: %v", err), "", 0
	}
	config := mapValues["config"]
	valued,ok := config.(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("config error"), "", 0
	}
	connect,_ := valued["connect"].(string)
	envId,_ := valued["envId"].(int)
	return nil, connect, envId
}


func cmForChart(chart *chart.Chart) (string, error) {
	results := []string{}
	if err := labelChartsConfigMap(chart, &results); err != nil {
		return  "", err
	}
	return mergeConfigMap(results), nil
}

func getValuesMap (cms string,vMap map[string]string) map[string]string {
	keyIndex :=  strings.Index(cms, "{{")
	if  keyIndex == -1{
		return vMap
	}

	valueIndex :=  strings.Index(cms, "}}")
	value := cms[keyIndex:valueIndex+2]

	key := cms[:keyIndex]
	key = strings.Replace(key," ","",-1)
	key = strings.Replace(key,"\n","",-1)
	if len(key) >=5 {
		vMap[key[len(key)-5:]] = value
	}
	newCms := cms[valueIndex+2:]
	return getValuesMap(newCms, vMap)
}

func removeValues(values map[string]interface{})  {
	for key, value := range values {
		if valued,ok := value.(string);  ok {
			if matched,_ := regexp.MatchString(".*{{.*}}.*", valued); matched {
				values[key] = tmpMagicStr
			}
		} else if valued,ok := value.(map[string]interface{});  ok{
			removeValues(valued)
		}

	}
}

func replaceValue(manifest string,valueMap map[string]string) string {
	index := strings.Index(manifest, tmpMagicStr)
	if index != -1 && index >= 8{
		key := manifest[0:index]
		key = strings.Replace(key," ","",-1)
		key = strings.Replace(key,"\n","",-1)
		replaceTo := "test: test"
		if len(key) >=5  && valueMap[key[len(key)-5:]] != "" {
			value := valueMap[key[len(key)-5:]]
			if indent := calculateIndent(value); indent != -1 {
				manifest = manifest[:index-indent] + manifest[index:]
			}
			manifest = strings.Replace(manifest,tmpMagicStr, valueMap[key[len(key)-5:]], 1)
			delete(valueMap, key)
		}else {
			manifest = strings.Replace(manifest,tmpMagicStr, replaceTo, 1)
		}
	}else {
		return manifest
	}
	return replaceValue(manifest, valueMap)
}

func calculateIndent(tmpStr string) int{
	idx := strings.Index(tmpStr,"indent")
	if idx == -1 {
		return -1
	}
	rest := tmpStr[idx+6:]
	str := strings.TrimSpace(rest)
	strs := strings.Split(str, " ")
	count := strs[0]
	b,error := strconv.Atoi(count)
	if	error != nil {
		return -1
	}
	return b
}



func removeStringValues(values string) (error, string) {
	mapValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(values), mapValues); err != nil {
		return fmt.Errorf("unmarshal values err: %v", err), ""
	}
	removeValues(mapValues)
	bytesValues,err := yaml.Marshal(mapValues)
	if  err != nil {
		return fmt.Errorf("marshal map values err: %v", err), ""
	}
	return nil, string(bytesValues)
}


func labelChartsConfigMap(chart *chart.Chart, results *[]string ) (error){
	for _,tmp := range chart.Templates {
		tmpContent := string(tmp.Data)
		if strings.Contains(tmpContent, "ConfigMap") {
			if strings.Contains(tmpContent, "---\n") {
				resources := strings.Split(tmpContent, "---\n")
				for _,resource := range resources {
					if strings.Contains(resource, "ConfigMap") {
						*results = append(*results, resource)
					}
				}

			} else {
				*results = append(*results, tmpContent)
			}

		}
	}
	for _,dependentChart := range chart.Dependencies {
		err :=  labelChartsConfigMap(dependentChart, results)
		if	err != nil {
			return err
		}
	}

	return  nil
}


func labelConfigMap(tmp string, releaseName string ) (string, error) {
	tmpMap := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(tmp), tmpMap); err != nil {
		return "", fmt.Errorf("label configMap error: %v", err)
	}
	metadata := tmpMap["metadata"]
	valuedMetadata, ok := metadata.(map[string]interface{})
	if !ok {
		return "", errors.New("label configMap error")
	}
	if valuedMetadata["labels"] == nil {
		labels := make(map[string]string)
		labels[model.ReleaseLabel] = releaseName
		valuedMetadata["labels"] = labels

	} else {
		labels := valuedMetadata["labels"]
		valuedMap, ok := labels.(map[string]string)
		if !ok {
			return "", errors.New("label configMap error")
		}
		valuedMap[model.ReleaseLabel] = releaseName
	}
	data := []byte{}
	if err := yaml.Unmarshal(data, tmpMap); err != nil {
		return "", fmt.Errorf("label configMap error: %v", err)
	}
	return string(data), nil
}

func mergeConfigMap(cms []string) string {
	result := ""
	for _,cm := range cms {
		result = "\n---\n" + cm
	}
	return result
}
