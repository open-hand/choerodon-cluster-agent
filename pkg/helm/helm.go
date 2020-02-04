package helm

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubectl"
	"github.com/choerodon/choerodon-cluster-agent/pkg/prometheus"
	"io/ioutil"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation"
	"k8s.io/client-go/rest"
	"os"
	"os/exec"
	"regexp"
	"strings"
	"text/template"

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
	"gopkg.in/yaml.v2"
	"strconv"
)

const (
	notesFileSuffix = "NOTES.txt"
	testNamespace   = "choerodon-test"
	tmpMagicStr     = "crockitwoodzZ: heihei"
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
		hooks.CRDInstall:         release.Hook_CRD_INSTALL,
	}

	expectedResourceKind = []string{"Deployment", "ReplicaSet", "Pod"}
)

type Client interface {
	ListRelease(namespace string) ([]*Release, error)
	ExecuteTest(request *TestReleaseRequest) (*TestReleaseResponse, error)
	InstallRelease(request *InstallReleaseRequest) (*Release, error)
	PreInstallRelease(request *InstallReleaseRequest) ([]*ReleaseHook, error)
	PreUpgradeRelease(request *UpgradeReleaseRequest) ([]*ReleaseHook, error)
	UpgradeRelease(request *UpgradeReleaseRequest) (*Release, error)
	RollbackRelease(request *RollbackReleaseRequest) (*Release, error)
	DeleteRelease(request *DeleteReleaseRequest) (*Release, error)
	StartRelease(request *StartReleaseRequest) (*StartReleaseResponse, error)
	StopRelease(request *StopReleaseRequest) (*StopReleaseResponse, error)
	GetReleaseContent(request *GetReleaseContentRequest) (*Release, error)
	GetRelease(request *GetReleaseContentRequest) (*Release, error)
	DeleteNamespaceReleases(namespaces string) error
}

type client struct {
	config     *rest.Config
	helmClient *helm.Client
	kubeClient envkube.Client
}

func init() {
	settings.AddFlags(pflag.CommandLine)
}

func NewClient(kubeClient envkube.Client, config *rest.Config) Client {
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
		config:     config,
		helmClient: helmClient,
		kubeClient: kubeClient,
	}
}

func (c *client) ListRelease(namespace string) ([]*Release, error) {
	releases := make([]*Release, 0)
	hlr, err := c.helmClient.ListReleases(helm.ReleaseListNamespace(namespace))
	if err != nil {
		glog.Error("helm client list release error", err)
	}

	for _, hr := range hlr.Releases {
		re := &Release{
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

func (c *client) PreInstallRelease(request *InstallReleaseRequest) ([]*ReleaseHook, error) {
	var releaseHooks []*ReleaseHook
    //查看release 是否存在。
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp != nil {
		return nil, fmt.Errorf("release %s already exist", request.ReleaseName)
	}
	//如果不存在那么
	//下载chart包到本地
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

func (c *client) InstallRelease(request *InstallReleaseRequest) (*Release, error) {
	// 查看指定名称的helm实例是否存在
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp != nil {
		return nil, fmt.Errorf("release %s already exist", request.ReleaseName)
	}

	// release名字不应该是DNS1123Subdomain
	if msgs := validation.IsDNS1123Subdomain(request.ReleaseName); request.ReleaseName != "" && len(msgs) > 0 {
		return nil, fmt.Errorf("release name %s is invalid: %s", request.ReleaseName, strings.Join(msgs, ";"))
	}
	//todo username and password

	// 查找chart包的位置
	cp, err := locateChartPath(request.RepoURL, "", "", request.ChartName, request.ChartVersion, false, "",
		"", "", "")
	if err != nil {
		return nil, err
	}
	chartRequested, err := chartutil.Load(cp)
	if err != nil {
		return nil, err
	}
	// 移除未启用的chart依赖
	chartutil.ProcessRequirementsEnabled(chartRequested, &chart.Config{Raw: request.Values})
    //request中是提交的vlaues
    //chartRequested中的是原来chart包中的values

	cm, err := cmForChart(chartRequested)
	if err != nil {
		return nil, fmt.Errorf("install chart error: %v", err)
	}
	valuesMap := make(map[string]string)
	valuesMap = getValuesMap(cm, valuesMap)

	// chart包的vlaues
	oldValues := chartRequested.Values.Raw
    //这一步将chart包中的vlaues去除注释
	err, newChartValues := removeStringValues(chartRequested.Values.Raw)
	if err != nil {
		return nil, err
	}
	//这一步将请求的vlaues去除注释
	err, newValues := removeStringValues(request.Values)
	if err != nil {
		return nil, err
	}
	chartRequested.Values.Raw = newChartValues

	if request.ChartName == "prometheus-operator" {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectlPath)
		kubectlApplier := kubectl.NewKubectl(kubectlPath, c.config)

		kubectlApplier.DeletePrometheusCrd()
	}

	// 获得经过渲染的文件名和文件内容
	files, _ := c.renderFiles(request.Namespace,
		chartRequested,
		request.ReleaseName,
		newValues,
		1)

	newTemplates := []*chart.Template{}

	for filename, f := range files {
		newManifestBuf, err := c.kubeClient.LabelObjects(request.Namespace,
			request.Command,
			request.ImagePullSecrets,
			f,
			request.ReleaseName,
			request.ChartName,
			request.ChartVersion,
			request.AppServiceId,
		)
		var newFile []byte
		if err != nil {
			//return nil, fmt.Errorf("label objects: %v", err)
			newFile = []byte(f)
		} else {
			newFile = []byte(replaceValue(string(newManifestBuf), valuesMap))
		}

		//add {{ }} when get {{ }}
		var re = regexp.MustCompile(`(\{\{[^}^}]*\}\})`)
		b := re.ReplaceAll(newFile, []byte("{{`$1`}}"))

		// 兼容prometheus的添加标签后不能正确解析模板
		if filename == "prometheus-operator/templates/prometheus/rules-1.14/prometheus.yaml" {
			t := prometheus.PrometheusRule{}
			err = yaml.Unmarshal(b, &t)
			if err != nil {
				glog.Info(err.Error())
			}
			t.ApiVersion = "monitoring.coreos.com/v1"
			t.Spec.Groups[0].Rules[12].Annotations["description"] = "Prometheus {{`{{$labels.namespace}}`}}/{{`{{$labels.pod}}`}} remote write desired shards calculation wants to run {{`{{ printf $value }}`}} shards, which is more than the max of {{print `{{ printf ` \"`\" `prometheus_remote_storage_shards_max{instance=\"%s\",job=\"{{ $prometheusJob }}\",namespace=\"{{ $namespace }}\"}` \"`\" ` $labels.instance | query | first | value }}`}}."
			result, err := yaml.Marshal(t)
			if err != nil {
				glog.Info(err.Error())
			}
			b = result
		}
		newTemplate := &chart.Template{Name: filename, Data: b}
		newTemplates = append(newTemplates, newTemplate)
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
				c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
				return nil, err
			}
			return rls, newError
		}
		c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
		return nil, newError
	}
	rls, err := c.getHelmRelease(installReleaseResp.GetRelease())
	rls.Command = request.Command
	rls.Commit = request.Commit
	if err != nil {
		return nil, err
	}
	if rls.Name == "choerodon-cert-manager" {
		kubectlPath, err := exec.LookPath("kubectl")
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectlPath)
		kubectlApplier := kubectl.NewKubectl(kubectlPath, c.config)

		err = kubectlApplier.ApplySingleObj("choerodon", getCertManagerIssuerData())
	}
	//数据量太大

	if rls.ChartName == "prometheus-operator" {
		rls.Hooks = nil
		rls.Resources = nil
	}
	return rls, err
}

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

func (c *client) ExecuteTest(request *TestReleaseRequest) (*TestReleaseResponse, error) {

	chartRequested, err := getChart(request.RepoURL, request.ChartName, request.ChartVersion)
	if err != nil {
		return nil, fmt.Errorf("load chart: %v", err)
	}

	chartutil.ProcessRequirementsEnabled(chartRequested, &chart.Config{Raw: request.Values})

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
	for _, hook := range hooks {
		manifestDocs = append(manifestDocs, hook.Manifest)
	}

	for index, manifestToInsert := range manifestDocs {
		newManifestBuf, err := c.kubeClient.LabelTestObjects(testNamespace, request.ImagePullSecrets, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion, request.Label)
		if err != nil {
			return nil, fmt.Errorf("label objects: %v", err)
		}
		if index == 0 {
			newTemplate := &chart.Template{Name: request.ReleaseName, Data: newManifestBuf.Bytes()}
			newTemplates = append(newTemplates, newTemplate)
		} else {
			newTemplate := &chart.Template{Name: "hook" + strconv.Itoa(index), Data: newManifestBuf.Bytes()}
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

	resp := &TestReleaseResponse{ReleaseName: request.ReleaseName}
	if err != nil {
		newError := fmt.Errorf("execute test release %s: %v", request.ReleaseName, err)
		if installReleaseResp != nil {
			_, err := c.getHelmRelease(installReleaseResp.GetRelease())
			if err != nil {
				c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
				return nil, err
			}

			return resp, newError
		}
		c.DeleteRelease(&DeleteReleaseRequest{ReleaseName: request.ReleaseName})
		return nil, newError
	}
	_, err = c.getHelmRelease(installReleaseResp.GetRelease())
	if err != nil {
		return nil, err
	}
	return resp, err
}

func (c *client) GetResources(namespace string, manifest string) ([]*ReleaseResource, error) {
	resources := make([]*ReleaseResource, 0, 10)
	result, err := c.kubeClient.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	var objPods = make(map[string][]v1.Pod)
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

		if err != nil {
			glog.Error("Warning: get the relation pod is failed, err:%s", err.Error())
		}
		objB, err := json.Marshal(info.Object)

		if err == nil {
			hrr.Object = string(objB)
		} else {
			glog.Error(err)
		}

		resources = append(resources, hrr)
		objPods, err = c.kubeClient.GetSelectRelationPod(info, objPods)
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

func (c *client) getHelmRelease(release *release.Release) (*Release, error) {
	resources, err := c.GetResources(release.Namespace, release.Manifest)
	if err != nil {
		return nil, fmt.Errorf("get resource: %v", err)
	}
	rlsHooks := make([]*ReleaseHook, len(release.GetHooks()))
	for i := 0; i < len(rlsHooks); i++ {
		rlsHook := release.GetHooks()[i]
		rlsHooks[i] = &ReleaseHook{
			Name: rlsHook.Name,
			Kind: rlsHook.Kind,
		}
	}
	rls := &Release{
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

func (c *client) getHelmReleaseNoResource(release *release.Release) (*Release, error) {
	rlsHooks := make([]*ReleaseHook, len(release.GetHooks()))
	for i := 0; i < len(rlsHooks); i++ {
		rlsHook := release.GetHooks()[i]
		rlsHooks[i] = &ReleaseHook{
			Name: rlsHook.Name,
			Kind: rlsHook.Kind,
		}
	}
	rls := &Release{
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

func (c *client) PreUpgradeRelease(request *UpgradeReleaseRequest) ([]*ReleaseHook, error) {
	var releaseHooks []*ReleaseHook

	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		installReq := &InstallReleaseRequest{
			RepoURL:          request.RepoURL,
			ChartName:        request.ChartName,
			ChartVersion:     request.ChartVersion,
			Values:           request.Values,
			ReleaseName:      request.ReleaseName,
			Namespace:        request.Namespace,
			AppServiceId:     request.AppServiceId,
			ImagePullSecrets: request.ImagePullSecrets,
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

func (c *client) UpgradeRelease(request *UpgradeReleaseRequest) (*Release, error) {
	releaseContentResp, err := c.helmClient.ReleaseContent(request.ReleaseName)
	if err != nil && !strings.Contains(err.Error(), ErrReleaseNotFound(request.ReleaseName).Error()) {
		return nil, err
	}
	if releaseContentResp == nil {
		installReq := &InstallReleaseRequest{
			RepoURL:          request.RepoURL,
			ChartName:        request.ChartName,
			ChartVersion:     request.ChartVersion,
			Values:           request.Values,
			ReleaseName:      request.ReleaseName,
			Namespace:        request.Namespace,
			Command:          request.Command, // 多填
			AppServiceId:     request.AppServiceId,
			ImagePullSecrets: request.ImagePullSecrets,
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

	chartutil.ProcessRequirementsEnabled(chartRequested, &chart.Config{Raw: request.Values})

	var hooks []*release.Hook
	var manifestDoc *bytes.Buffer
	manifestDocs := []string{}
	newTemplates := []*chart.Template{}

	hooks, manifestDoc, err = c.renderManifests(
		request.Namespace,
		chartRequested,
		request.ReleaseName,
		request.Values,
		1)
	if err != nil {
		glog.V(1).Infof("sort error...")
		return nil, err
	}

	if manifestDoc != nil {
		manifestDocs = append(manifestDocs, manifestDoc.String())
	}
	for _, hook := range hooks {
		manifestDocs = append(manifestDocs, hook.Manifest)
	}
	// 如果是agent升级，则跳过添加标签这一步，因为agent原本是直接在集群中安装的没有对应标签，如果在这里加标签k8s会报错
	if request.ChartName != "choerodon-cluster-agent" {
		for index, manifestToInsert := range manifestDocs {
			var newManifestBuf []byte
			var err error
			if strings.Contains(request.ChartName, "prometheus-operator") {
				newManifestBuf, err = c.kubeClient.LabelObjectsForPrometheusUpdate(request.Namespace, request.Command, request.ImagePullSecrets, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion, request.AppServiceId)
			} else {
				newManifestBuf, err = c.kubeClient.LabelObjects(request.Namespace, request.Command, request.ImagePullSecrets, manifestToInsert, request.ReleaseName, request.ChartName, request.ChartVersion, request.AppServiceId)
			}
			if err != nil {
				return nil, fmt.Errorf("label objects: %v", err)
			}
			if index == 0 {
				newTemplate := &chart.Template{Name: request.ReleaseName, Data: newManifestBuf}
				newTemplates = append(newTemplates, newTemplate)
			} else {
				newTemplate := &chart.Template{Name: "hook" + strconv.Itoa(index), Data: newManifestBuf}
				newTemplates = append(newTemplates, newTemplate)
			}
			chartRequested.Templates = newTemplates
			chartRequested.Dependencies = []*chart.Chart{}
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
			//这里先不加command
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
	rls.Command = request.Command
	if rls.ChartName == "prometheus-operator" {
		rls.Hooks = nil
		rls.Resources = nil
	}
	return rls, nil
}

func (c *client) RollbackRelease(request *RollbackReleaseRequest) (*Release, error) {
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

func (c *client) DeleteRelease(request *DeleteReleaseRequest) (*Release, error) {
	deleteReleaseResp, err := c.helmClient.DeleteRelease(
		request.ReleaseName,
		helm.DeletePurge(true),
	)
	if strings.Contains(request.ReleaseName, "prometheus-operator") {
		c.kubeClient.GetKubeClient().ExtensionsV1beta1().RESTClient().Delete().Name("memcacheds.cache.example.com").Resource("CustomResourceDefinition")
	}

	if err != nil {
		return nil, fmt.Errorf("delete release %s: %v", request.ReleaseName, err)
	}
	rls, err := c.getHelmRelease(deleteReleaseResp.Release)
	if err != nil {
		return nil, err
	}
	return rls, nil
}

func (c *client) StopRelease(request *StopReleaseRequest) (*StopReleaseResponse, error) {
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
	resp := &StopReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

func (c *client) StartRelease(request *StartReleaseRequest) (*StartReleaseResponse, error) {
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
	resp := &StartReleaseResponse{
		ReleaseName: request.ReleaseName,
	}
	return resp, nil
}

func (c *client) renderFiles(
	namespace string,
	chartRequested *chart.Chart,
	releaseName string,
	values string,
	revision int) (map[string]string, error) {
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

	// 获取kube发现接口
	discoveryInterface, err := c.kubeClient.GetDiscoveryClient()
	if err != nil {
		return nil, err
	}

	// 获取kubeapi版本、集群版本、tiller版本
	caps, err := capabilities(discoveryInterface)

	// 获得渲染template的数据结构
	valuesToRender, err := chartutil.ToRenderValuesCaps(chartRequested, valuesConfig, options, caps)
	if err != nil {
		return nil, err
	}
	if err != nil {
		glog.V(1).Infof("unmarshal error...")
		return nil, err
	}

	renderer := env.EngineYard.Default()
	if chartRequested.Metadata.Engine != "" {
		if r, ok := env.EngineYard.Get(chartRequested.Metadata.Engine); ok {
			renderer = r
		} else {
			glog.Infof("warning: %s requested non-existent template engine %s", chartRequested.Metadata.Name, chartRequested.Metadata.Engine)
		}
	}

	// 获得经过渲染的文件名和文件内容
	return renderer.Render(chartRequested, valuesToRender)
}

func (c *client) renderManifests(
	namespace string,
	chartRequested *chart.Chart,
	releaseName string,
	values string,
	revision int) ([]*release.Hook, *bytes.Buffer, error) {
	files, err := c.renderFiles(namespace, chartRequested, releaseName, values, revision)
	discoveryInterface, err := c.kubeClient.GetDiscoveryClient()
	caps, err := capabilities(discoveryInterface)
	if err != nil {
		return nil, nil, err
	}
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

func (c *client) GetReleaseContent(request *GetReleaseContentRequest) (*Release, error) {
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

	rlss, err := c.helmClient.ListReleases(helm.ReleaseListNamespace(namespaces))
	if err != nil {
		glog.Errorf("delete ns release failed %v", err)
		return err
	}
	if rlss == nil {
		return nil
	}
	for _, rls := range rlss.Releases {
		c.helmClient.DeleteRelease(rls.Name, helm.DeletePurge(true))
	}
	return nil

}

func (c *client) GetRelease(request *GetReleaseContentRequest) (*Release, error) {
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

func getEnvInfo(values string) (error, string, int) {
	mapValues := make(map[string]interface{})
	if err := yaml.Unmarshal([]byte(values), mapValues); err != nil {
		return fmt.Errorf("unmarshal values err: %v", err), "", 0
	}
	config := mapValues["config"]
	valued, ok := config.(map[interface{}]interface{})
	if !ok {
		return fmt.Errorf("config error"), "", 0
	}
	connect, _ := valued["connect"].(string)
	envId, _ := valued["envId"].(int)
	return nil, connect, envId
}

func cmForChart(chart *chart.Chart) (string, error) {
	results := []string{}
	// 取出所有的ConfigMap对象
	if err := labelChartsConfigMap(chart, &results); err != nil {
		return "", err
	}

	//将所有的ConfigMap对象进行合并
	return mergeConfigMap(results), nil
}

func getValuesMap(cms string, vMap map[string]string) map[string]string {
	keyIndex := strings.Index(cms, "{{")
	if keyIndex == -1 {
		return vMap
	}

	valueIndex := strings.Index(cms, "}}")
	for valueIndex < keyIndex {
		cms = cms[valueIndex+2:]
		valueIndex = strings.Index(cms, "}}")
	}
	value := cms[keyIndex : valueIndex+2]

	key := cms[:keyIndex]
	key = strings.Replace(key, " ", "", -1)
	key = strings.Replace(key, "\n", "", -1)
	if len(key) >= 5 {
		vMap[key[len(key)-5:]] = value
	}
	newCms := cms[valueIndex+2:]
	return getValuesMap(newCms, vMap)
}

func removeValues(values map[string]interface{}) {
	for key, value := range values {
		if valued, ok := value.(string); ok {
			if matched, _ := regexp.MatchString(".*{{.*}}.*", valued); matched {
				values[key] = tmpMagicStr
			}
		} else if valued, ok := value.(map[string]interface{}); ok {
			removeValues(valued)
		}

	}
}

func replaceValue(manifest string, valueMap map[string]string) string {
	index := strings.Index(manifest, tmpMagicStr)
	if index != -1 && index >= 8 {
		key := manifest[0:index]
		key = strings.Replace(key, " ", "", -1)
		key = strings.Replace(key, "\n", "", -1)
		replaceTo := "test: test"
		if len(key) >= 5 && valueMap[key[len(key)-5:]] != "" {
			value := valueMap[key[len(key)-5:]]
			if indent := calculateIndent(value); indent != -1 {
				manifest = manifest[:index-indent] + manifest[index:]
			}
			manifest = strings.Replace(manifest, tmpMagicStr, valueMap[key[len(key)-5:]], 1)
			delete(valueMap, key)
		} else {
			manifest = strings.Replace(manifest, tmpMagicStr, replaceTo, 1)
		}
	} else {
		return manifest
	}
	return replaceValue(manifest, valueMap)
}

func calculateIndent(tmpStr string) int {
	idx := strings.Index(tmpStr, "indent")
	if idx == -1 {
		return -1
	}
	rest := tmpStr[idx+6:]
	str := strings.TrimSpace(rest)
	strs := strings.Split(str, " ")
	count := strs[0]
	b, error := strconv.Atoi(count)
	if error != nil {
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
	bytesValues, err := yaml.Marshal(mapValues)
	if err != nil {
		return fmt.Errorf("marshal map values err: %v", err), ""
	}
	return nil, string(bytesValues)
}

func labelChartsConfigMap(chart *chart.Chart, results *[]string) error {
	for _, tmp := range chart.Templates {
		tmpContent := string(tmp.Data)
		if strings.Contains(tmpContent, "ConfigMap") {
			if strings.Contains(tmpContent, "---\n") {
				resources := strings.Split(tmpContent, "---\n")
				for _, resource := range resources {
					if strings.Contains(resource, "ConfigMap") {
						*results = append(*results, resource)
					}
				}

			} else {
				*results = append(*results, tmpContent)
			}

		}
	}
	for _, dependentChart := range chart.Dependencies {
		err := labelChartsConfigMap(dependentChart, results)
		if err != nil {
			return err
		}
	}

	return nil
}

func labelConfigMap(tmp string, releaseName string) (string, error) {
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
	for _, cm := range cms {
		result = result + "\n---\n" + cm
	}
	return strings.Trim(result, "\n---\n")
}

func inArray(expectedResourceKind []string, kind string) bool {
	for _, item := range expectedResourceKind {
		if item == kind {
			return true
		}
	}
	return false
}
