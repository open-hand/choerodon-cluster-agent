package helm

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	envkube "github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubectl"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"github.com/open-hand/helm/pkg/action"
	"github.com/open-hand/helm/pkg/chart"
	"github.com/open-hand/helm/pkg/chart/loader"
	"github.com/open-hand/helm/pkg/cli"
	"github.com/open-hand/helm/pkg/cli/values"
	"github.com/open-hand/helm/pkg/downloader"
	"github.com/open-hand/helm/pkg/getter"
	"github.com/open-hand/helm/pkg/release"
	"github.com/pkg/errors"
	"html/template"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/client-go/rest"
	"log"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
)

const (
	ErrReleaseNotFound = "release: not found"
	TestNamespace      = "choerodon-test"
)

var (
	ExpectedResourceKind = []string{"Deployment", "ReplicaSet", "StatefulSet"}
)

type Client interface {
	ListRelease(namespace string) ([]*Release, error)
	ExecuteTest(request *TestReleaseRequest, username, password string) (*TestReleaseResponse, error)
	InstallRelease(request *InstallReleaseRequest, username, password string) (*Release, error)
	PreInstallRelease(request *InstallReleaseRequest, username, password string) ([]*ReleaseHook, error)
	PreUpgradeRelease(request *UpgradeReleaseRequest, username, password string) ([]*ReleaseHook, error)
	UpgradeRelease(request *UpgradeReleaseRequest, username, password string) (*Release, error)
	DeleteRelease(request *DeleteReleaseRequest) (*release.UninstallReleaseResponse, error)
	StartRelease(request *StartReleaseRequest, cluster *kubernetes.Cluster) (*StartReleaseResponse, error)
	StopRelease(request *StopReleaseRequest, cluster *kubernetes.Cluster) (*StopReleaseResponse, error)
	GetRelease(request *GetReleaseContentRequest) (*Release, error)
	DeleteNamespaceReleases(namespaces string) error
	//GetReleaseContent(request *GetReleaseContentRequest) (*Release, error)
	//RollbackRelease(request *RollbackReleaseRequest) (*Release, error)
	GetKubeClient() (envkube.Client, error)
}

type client struct {
	config     *rest.Config
	kubeClient envkube.Client
}

func debug(format string, v ...interface{}) {
	format = fmt.Sprintf("[helm debug] %s\n", format)
	glog.V(1).Info(fmt.Sprintf(format, v...))
}

func NewClient(kubeClient envkube.Client, config *rest.Config) Client {
	return &client{
		config:     config,
		kubeClient: kubeClient,
	}
}

// 列出指定命名空间下的所有实例
func (c *client) ListRelease(namespace string) ([]*Release, error) {
	cfg, _ := getCfg(namespace)
	listClient := action.NewList(cfg)
	rels := make([]*Release, 0)
	listResponseRelease, err := listClient.Run()
	if err != nil {
		glog.Error("helm client list release error", err)
	}

	for _, response := range listResponseRelease {

		releaseConifg, err := yaml.Marshal(response.Config)
		if err != nil {
			return nil, err
		}

		re := &Release{
			Namespace:    namespace,
			Name:         response.Name,
			Revision:     response.Version,
			Status:       response.Info.Status.String(),
			ChartName:    response.Chart.Metadata.Name,
			ChartVersion: response.Chart.Metadata.Version,
			Config:       string(releaseConifg),
		}
		rels = append(rels, re)
	}
	return rels, nil
}

// 这一步操作存粹是为了获得hooks
func (c *client) PreInstallRelease(request *InstallReleaseRequest, username, password string) ([]*ReleaseHook, error) {
	var releaseHooks []*ReleaseHook

	chartPathOptions := action.ChartPathOptions{
		RepoURL:  request.RepoURL,
		Version:  request.ChartVersion,
		Username: username,
		Password: password,
	}

	cfg, envSettings := getCfg(request.Namespace)

	// 用来获取hooks
	showClient := action.NewShow(cfg, "", chartPathOptions)
	// 用来获取release
	getClient := action.NewGet(cfg)
	//查看release 是否存在。
	releaseContentResp, err := getClient.Run(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if releaseContentResp != nil {
		return nil, fmt.Errorf("release %s already exist", request.ReleaseName)
	}
	//如果不存在那么
	//下载chart包到本地
	cp, err := showClient.LocateChart(request.ChartName, envSettings)
	if err != nil {
		return nil, err
	}
	chrt, err := loader.Load(cp)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	valueOpts := getValueOpts(request.Values)
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	hooks, err := showClient.FindHooks(chrt, vals)

	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	for _, hook := range hooks {
		releaseHook := &ReleaseHook{
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

// 安装实例
func (c *client) InstallRelease(request *InstallReleaseRequest, username, password string) (*Release, error) {
	chartPathOptions := action.ChartPathOptions{
		RepoURL:  request.RepoURL,
		Version:  request.ChartVersion,
		Username: username,
		Password: password,
	}

	cfg, envSettings := getCfg(request.Namespace)

	installClient := action.NewInstall(
		cfg,
		chartPathOptions,
		request.Command,
		request.ImagePullSecrets,
		request.Namespace,
		request.ReleaseName,
		request.ChartName,
		request.ChartVersion,
		request.AppServiceId,
		model.AgentVersion,
		"",
		false)

	// 如果是安装prometheus-operator，则先删掉集群中的prometheus相关crd
	if request.ChartName == "prometheus-operator" {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectlPath)
		kubectlApplier := kubectl.NewKubectl(kubectlPath, c.config)

		kubectlApplier.DeletePrometheusCrd()
	}

	cp, err := installClient.ChartPathOptions.LocateChart(request.ChartName, envSettings)
	if err != nil {
		return nil, err
	}

	glog.V(1).Infof("CHART PATH: %s\n", cp)

	valueOpts := getValueOpts(request.Values)
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	validInstallableChart, err := isChartInstallable(chartRequested)
	if !validInstallableChart {
		return nil, err
	}

	if chartRequested.Metadata.Deprecated {
		glog.Info("WARNING: This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if installClient.DependencyUpdate {
				man := &downloader.Manager{
					ChartPath:        cp,
					Keyring:          installClient.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: envSettings.RepositoryConfig,
					RepositoryCache:  envSettings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	responseRelease, err := installClient.Run(chartRequested, vals, request.Values)
	if err != nil {
		return nil, err
	}

	rls, err := c.getHelmRelease(responseRelease)
	rls.Command = request.Command
	rls.Commit = request.Commit
	if err != nil {
		return nil, err
	}
	if rls.Name == "choerodon-cert-manager" {
		// 睡眠等待3秒，防止创建cert-manager对象的时候crd还没创建成功
		time.Sleep(3 * time.Second)
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectlPath)
		kubectlApplier := kubectl.NewKubectl(kubectlPath, c.config)

		err = kubectlApplier.ApplySingleObj("choerodon", getCertManagerIssuerData())
	}
	//数据量太大

	if rls.ChartName == "prometheus-operator" || rls.ChartName == "cert-manager" {
		rls.Hooks = nil
		rls.Resources = nil
	}
	return rls, err
}

// 获得certmanager颁发者数据
func getCertManagerIssuerData() string {
	email := os.Getenv("ACME_EMAIL")
	if email == "" {
		email = "change_it@choerodon.io"
	}
	tml, err := template.New("issuertpl").Parse(model.CertManagerClusterIssuer)
	if err != nil {
		glog.Error(err)
	}
	var data bytes.Buffer
	if err := tml.Execute(&data, struct {
		ACME_EMAIL string
	}{
		ACME_EMAIL: email,
	}); err != nil {
		glog.Error(err)
	}
	return data.String()
}

// 执行测试
func (c *client) ExecuteTest(request *TestReleaseRequest, username, password string) (*TestReleaseResponse, error) {

	chartPathOptions := action.ChartPathOptions{
		RepoURL:  request.RepoURL,
		Version:  request.ChartVersion,
		Username: username,
		Password: password,
	}

	cfg, envSettings := getCfg(TestNamespace)

	installClient := action.NewInstall(
		cfg,
		chartPathOptions,
		0,
		request.ImagePullSecrets,
		TestNamespace,
		request.ReleaseName,
		request.ChartName,
		request.ChartVersion,
		0,
		model.AgentVersion,
		request.Label,
		true)

	cp, err := installClient.ChartPathOptions.LocateChart(request.ChartName, envSettings)
	if err != nil {
		return nil, err
	}

	glog.V(1).Infof("CHART PATH: %s\n", cp)

	valueOpts := getValueOpts(request.Values)
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	chartRequested, err := loader.Load(cp)
	if err != nil {
		return nil, err
	}

	validInstallableChart, err := isChartInstallable(chartRequested)
	if !validInstallableChart {
		return nil, err
	}

	if chartRequested.Metadata.Deprecated {
		glog.Info("WARNING: This chart is deprecated")
	}

	if req := chartRequested.Metadata.Dependencies; req != nil {
		// If CheckDependencies returns an error, we have unfulfilled dependencies.
		// As of Helm 2.4.0, this is treated as a stopping condition:
		// https://github.com/helm/helm/issues/2209
		if err := action.CheckDependencies(chartRequested, req); err != nil {
			if installClient.DependencyUpdate {
				man := &downloader.Manager{
					ChartPath:        cp,
					Keyring:          installClient.ChartPathOptions.Keyring,
					SkipUpdate:       false,
					Getters:          p,
					RepositoryConfig: envSettings.RepositoryConfig,
					RepositoryCache:  envSettings.RepositoryCache,
				}
				if err := man.Update(); err != nil {
					return nil, err
				}
			} else {
				return nil, err
			}
		}
	}

	responseRelease, err := installClient.Run(chartRequested, vals, request.Values)

	resp := &TestReleaseResponse{ReleaseName: request.ReleaseName}
	if err != nil {
		newError := fmt.Errorf("execute test release %s: %v", request.ReleaseName, err)
		if responseRelease != nil {
			_, err := c.getHelmRelease(responseRelease)
			if err != nil {
				c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
				return nil, err
			}

			return resp, newError
		}
		c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
		return nil, newError
	}

	_, err = c.getHelmRelease(responseRelease)
	if err != nil {
		return nil, err
	}
	return resp, err
}

// TODO 获得资源信息包括pod信息，这里要注意从manifest获取，agent添加的相关label信息会丢失,需要测试是否有问题
func (c *client) GetResources(namespace string, manifest string) ([]*ReleaseResource, error) {
	resources := make([]*ReleaseResource, 0, 10)
	result, err := c.kubeClient.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	for _, info := range result {
		if err := info.Get(); err != nil {
			continue
		}
		hrr := &ReleaseResource{
			Group:           info.Object.GetObjectKind().GroupVersionKind().Group,
			Version:         info.Object.GetObjectKind().GroupVersionKind().Version,
			Kind:            info.Object.GetObjectKind().GroupVersionKind().Kind,
			Name:            info.Name,
			ResourceVersion: info.ResourceVersion,
		}

		objB, err := json.Marshal(info.Object)

		if err == nil {
			hrr.Object = string(objB)
		} else {
			return nil, err
		}

		resources = append(resources, hrr)
		objPods, err := c.kubeClient.GetSelectRelationPod(info)
		if err != nil {
			glog.Error("Warning: get the relation pod is failed, err:%s", err.Error())
		}
		//here, we will add the objPods to the objs
		for _, podItems := range objPods {
			for i := range podItems {
				hrr := &ReleaseResource{
					Group:           podItems[i].GroupVersionKind().Group,
					Version:         podItems[i].GroupVersionKind().Version,
					Kind:            podItems[i].GroupVersionKind().Kind,
					Name:            podItems[i].Name,
					ResourceVersion: podItems[i].ResourceVersion,
				}
				objPod, err := json.Marshal(podItems[i])
				if err == nil {
					hrr.Object = string(objPod)
				} else {
					glog.Error(err)
				}

				resources = append(resources, hrr)
			}
		}
	}
	return resources, nil
}

// 获得helmRelease
func (c *client) getHelmRelease(release *release.Release) (*Release, error) {
	resources, err := c.GetResources(release.Namespace, release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	rlsHooks := make([]*ReleaseHook, len(release.Hooks))
	for i := 0; i < len(rlsHooks); i++ {
		rlsHook := release.Hooks[i]
		rlsHooks[i] = &ReleaseHook{
			Name: rlsHook.Name,
			Kind: rlsHook.Kind,
		}
	}

	releaseConifg, err := yaml.Marshal(release.Config)
	if err != nil {
		return nil, err
	}

	rls := &Release{
		Name:         release.Name,
		Revision:     release.Version,
		Namespace:    release.Namespace,
		Status:       release.Info.Status.String(),
		ChartName:    release.Chart.Metadata.Name,
		ChartVersion: release.Chart.Metadata.Version,
		Manifest:     release.Manifest,
		Hooks:        rlsHooks,
		Resources:    resources,
		Config:       string(releaseConifg),
	}
	return rls, nil
}

// 获得资源信息，不要pod相关信息
func (c *client) getHelmReleaseNoResource(release *release.Release) (*Release, error) {
	rlsHooks := make([]*ReleaseHook, len(release.Hooks))
	for i := 0; i < len(rlsHooks); i++ {
		rlsHook := release.Hooks[i]
		rlsHooks[i] = &ReleaseHook{
			Name: rlsHook.Name,
			Kind: rlsHook.Kind,
		}
	}

	rls := &Release{
		Name:         release.Name,
		Revision:     release.Version,
		Namespace:    release.Namespace,
		Status:       release.Info.Status.String(),
		ChartName:    release.Chart.Metadata.Name,
		ChartVersion: release.Chart.Metadata.Version,
		Manifest:     release.Manifest,
		Hooks:        rlsHooks,
		Config:       release.ConfigRaw,
	}
	return rls, nil
}

// 升级实例前的预操作
func (c *client) PreUpgradeRelease(request *UpgradeReleaseRequest, username, password string) ([]*ReleaseHook, error) {
	var releaseHooks []*ReleaseHook

	chartPathOptions := action.ChartPathOptions{
		RepoURL:  request.RepoURL,
		Version:  request.ChartVersion,
		Username: username,
		Password: password,
	}

	cfg, envSettings := getCfg(request.Namespace)

	getClient := action.NewGet(cfg)
	showClient := action.NewShow(cfg, "", chartPathOptions)

	responseRelease, err := getClient.Run(request.ReleaseName)

	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if responseRelease == nil {
		installReleaseReq := &InstallReleaseRequest{
			RepoURL:          request.RepoURL,
			ChartName:        request.ChartName,
			ChartVersion:     request.ChartVersion,
			Values:           request.Values,
			ReleaseName:      request.ReleaseName,
			Namespace:        request.Namespace,
			AppServiceId:     request.AppServiceId,
			ImagePullSecrets: request.ImagePullSecrets,
		}
		return c.PreInstallRelease(installReleaseReq, username, password)
	}

	//如果不存在那么
	//下载chart包到本地
	cp, err := showClient.LocateChart(request.ChartName, envSettings)
	if err != nil {
		return nil, err
	}
	chrt, err := loader.Load(cp)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	valueOpts := getValueOpts(request.Values)
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}

	hooks, err := showClient.FindHooks(chrt, vals)

	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	for _, hook := range hooks {
		releaseHook := &ReleaseHook{
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

// 升级实例
func (c *client) UpgradeRelease(request *UpgradeReleaseRequest, username, password string) (*Release, error) {
	cfg, envSettings := getCfg(request.Namespace)

	getClient := action.NewGet(cfg)
	responseRelease, err := getClient.Run(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if responseRelease == nil {
		installReq := &InstallReleaseRequest{
			RepoURL:          request.RepoURL,
			ChartName:        request.ChartName,
			ChartVersion:     request.ChartVersion,
			Values:           request.Values,
			ReleaseName:      request.ReleaseName,
			Namespace:        request.Namespace,
			Command:          request.Command,
			AppServiceId:     request.AppServiceId,
			ImagePullSecrets: request.ImagePullSecrets,
		}
		installResp, err := c.InstallRelease(installReq, username, password)
		if err != nil {
			return nil, err
		}
		return installResp, nil
	}

	chartPathOptions := action.ChartPathOptions{
		RepoURL:  request.RepoURL,
		Version:  request.ChartVersion,
		Username: username,
		Password: password,
	}
	upgradeClient := action.NewUpgrade(
		cfg,
		chartPathOptions,
		request.Command,
		request.ImagePullSecrets,
		request.ReleaseName,
		request.ChartName,
		request.ChartVersion,
		request.AppServiceId,
		model.AgentVersion)

	chartPath, err := upgradeClient.ChartPathOptions.LocateChart(request.ChartName, envSettings)
	if err != nil {
		return nil, err
	}

	// Check chart dependencies to make sure all are present in /charts
	ch, err := loader.Load(chartPath)
	if err != nil {
		return nil, err
	}
	if req := ch.Metadata.Dependencies; req != nil {
		if err := action.CheckDependencies(ch, req); err != nil {
			return nil, err
		}
	}

	if ch.Metadata.Deprecated {
		glog.Warning("WARNING: This chart is deprecated")
	}

	valueOpts := getValueOpts(request.Values)
	p := getter.All(envSettings)
	vals, err := valueOpts.MergeValues(p)
	if err != nil {
		return nil, err
	}
	updateReleaseResp, err := upgradeClient.Run(request.ReleaseName, ch, vals, request.Values)

	if err != nil {
		newErr := fmt.Errorf("update release %s: %v", request.ReleaseName, err)
		if updateReleaseResp != nil {
			rls, err := c.getHelmRelease(updateReleaseResp)
			//这里先不加command
			if err != nil {
				return nil, err
			}
			return rls, newErr
		}
		return nil, newErr
	}

	rls, err := c.getHelmRelease(updateReleaseResp)
	if err != nil {
		return nil, err
	}
	rls.Commit = request.Commit
	rls.Command = request.Command
	if rls.ChartName == "prometheus-operator" || rls.ChartName == "cert-manager" {
		rls.Hooks = nil
		rls.Resources = nil
	}
	return rls, nil
}

// 删除实例
func (c *client) DeleteRelease(request *DeleteReleaseRequest) (*release.UninstallReleaseResponse, error) {
	cfg, _ := getCfg(request.Namespace)
	uninstall := action.NewUninstall(cfg)

	rls, err := uninstall.Run(request.ReleaseName)
	if rls != nil {
		if rls.Release.Chart.Metadata.Name == "prometheus-operator" || rls.Release.Chart.Metadata.Name == "cert-manager" {
			rls.Release.Hooks = nil
			rls.Release.Manifest = ""
		}
	}
	if err != nil {
		return nil, err
	}
	return rls, nil
}

// 停用实例
func (c *client) StopRelease(request *StopReleaseRequest, cluster *kubernetes.Cluster) (*StopReleaseResponse, error) {
	cfg, _ := getCfg(request.Namespace)
	getClient := action.NewGet(cfg)
	responseRelease, err := getClient.Run(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if responseRelease == nil {
		return nil, fmt.Errorf("release: not found")
	}

	result, err := c.kubeClient.BuildUnstructured(request.Namespace, responseRelease.Manifest)

	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	for _, info := range result {
		if envkube.InArray(ExpectedResourceKind, info.Object.GetObjectKind().GroupVersionKind().Kind) {
			err := cluster.ScaleResource(request.Namespace, info.Object.GetObjectKind().GroupVersionKind().Kind, info.Name, "0")
			if err != nil {
				return nil, fmt.Errorf("scale: %v", err)
			}
		}
	}

	resp := &StopReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

// 开启实例
func (c *client) StartRelease(request *StartReleaseRequest, cluster *kubernetes.Cluster) (*StartReleaseResponse, error) {
	cfg, _ := getCfg(request.Namespace)
	getClient := action.NewGet(cfg)
	responseRelease, err := getClient.Run(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if responseRelease == nil {
		return nil, fmt.Errorf("release: not found")
	}

	result, err := c.kubeClient.BuildUnstructured(request.Namespace, responseRelease.Manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	for _, info := range result {
		if envkube.InArray(ExpectedResourceKind, info.Object.GetObjectKind().GroupVersionKind().Kind) {
			t := info.Object.(*unstructured.Unstructured)
			replicas, found, err := unstructured.NestedInt64(t.Object, "spec", "replicas")
			if err != nil {
				return nil, fmt.Errorf("get Template replicas failed, %v", err)
			}
			if found == false {
				replicas = 1
			}
			err = cluster.ScaleResource(request.Namespace, info.Object.GetObjectKind().GroupVersionKind().Kind, info.Name, strconv.FormatInt(replicas, 10))
			if err != nil {
				return nil, fmt.Errorf("scale: %v", err)
			}
		}
	}

	resp := &StartReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

// 删除指定命名空间下的所有实例
func (c *client) DeleteNamespaceReleases(namespace string) error {

	rlss, err := c.ListRelease(namespace)
	if err != nil {
		glog.Errorf("delete ns release failed %v", err)
		return err
	}
	if rlss == nil {
		return nil
	}
	for _, rls := range rlss {
		deleteReleaseRequest := &DeleteReleaseRequest{
			ReleaseName: rls.Name,
			Namespace:   rls.Namespace,
		}
		c.DeleteRelease(deleteReleaseRequest)
	}
	return nil

}

func (c *client) GetKubeClient() (envkube.Client, error) {
	if c.kubeClient == nil {
		return nil, errors.New("no kube-client founded")
	}
	return c.kubeClient, nil
}

func (c *client) GetRelease(request *GetReleaseContentRequest) (*Release, error) {
	cfg, _ := getCfg(request.Namespace)
	getClient := action.NewGet(cfg)
	responseRelease, err := getClient.Run(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound) {
		return nil, err
	}
	if responseRelease == nil {
		return nil, fmt.Errorf("release: not found")
	}
	rls, err := c.getHelmReleaseNoResource(responseRelease)
	if err != nil {
		return nil, err
	}
	return rls, nil
}

// 校验chart包是否可安装
func isChartInstallable(ch *chart.Chart) (bool, error) {
	switch ch.Metadata.Type {
	case "", "application":
		return true, nil
	}
	return false, errors.Errorf("%s charts are not installable", ch.Metadata.Type)
}

// 获取helm的配置信息
func getCfg(namespace string) (*action.Configuration, *cli.EnvSettings) {
	settings := cli.New()
	settings.SetNamespace(namespace)
	actionConfig := &action.Configuration{}

	helmDriver := os.Getenv("HELM_DRIVER")
	if err := actionConfig.Init(settings.RESTClientGetter(), settings.Namespace(), helmDriver, debug); err != nil {
		log.Fatal(err)
	}
	return actionConfig, settings
}

func getValueOpts(value string) *values.Options {
	return &values.Options{
		ValuesFromRequest: value,
	}
}

// TODO devops 没有使用此功能，注释掉，等待以后业务再处理
//func (c *client) RollbackRelease(request *RollbackReleaseRequest) (*Release, error) {
//	rollbackReleaseResp, err := c.helmClient.RollbackRelease(
//		request.ReleaseName,
//		helm.RollbackVersion(int32(request.Version)))
//	if err != nil {
//		return nil, fmt.Errorf("rollback release %s: %v", request.ReleaseName, err)
//	}
//	rls, err := c.getHelmRelease(rollbackReleaseResp.Release)
//	if err != nil {
//		return nil, err
//	}
//
//	return rls, nil
//}
