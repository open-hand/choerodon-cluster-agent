package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/certificate/client/clientset/versioned"
	choerodon_version "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/clientset/versioned"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"io"
	core_v1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	cmdutil "k8s.io/kubectl/pkg/cmd/util"
	"k8s.io/kubectl/pkg/scheme"
	"net/http"
	"reflect"
	"strconv"
	"strings"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
)

type Client interface {
	DeleteJob(namespace string, name string) error
	LogsForJob(namespace string, name string, jobLabel string) (string, string, error)
	CreateOrUpdateService(namespace string, serviceStr string) (*core_v1.Service, error)
	CreateOrUpdateIngress(namespace string, ingressStr string) (*ext_v1beta1.Ingress, error)
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
	DeleteService(namespace string, name string) error
	DeleteIngress(namespace string, name string) error
	GetLogs(namespace string, pod string, container string) (io.ReadCloser, error)
	Exec(namespace string, podName string, containerName string, local io.ReadWriter) error
	LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error)
	GetService(namespace string, serviceName string) (string, error)
	GetIngress(namespace string, ingressName string) (string, error)
	GetConfigMap(namespace string, configMapName string) (string, error)
	GetPersistentVolumeClaim(namespace string, pvcName string) (string, string, error)
	GetPersistentVolume(namespace string, pvcName string) (string, string, error)
	GetNamespace(namespace string) error
	DeleteNamespace(namespace string) error
	GetSecret(namespace string, secretName string) (string, error)
	GetKubeClient() *kubernetes.Clientset
	GetCrdClient() *versioned.Clientset
	GetC7nClient() *choerodon_version.Clientset
	GetRESTConfig() (*rest.Config, error)
	IsReleaseJobRun(namespace, releaseName string) bool
	CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error)
	BuildUnstructured(namespace string, manifest string) (Result, error)
	//todo: delete follow func
	GetSelectRelationPod(info *resource.Info, objPods map[string][]core_v1.Pod) (map[string][]core_v1.Pod, error)
	GetPodByLabelSelector(info *resource.Info) (*v1.PodList, error)
	GetC7NHelmRelease(name, namespace string) *v1alpha1.C7NHelmRelease
}

const testContainer string = "automation-test"

var ClusterId int32
var KubernetesVersion string
var AgentVersion string
var expectedResourceKind = []string{"Deployment", "ReplicaSet", "ReplicationController", "StatefulSet"}

type client struct {
	cmdutil.Factory
	client    *kubernetes.Clientset
	crdClient *versioned.Clientset
	c7nClient *choerodon_version.Clientset
}

func NewClient(f cmdutil.Factory) (Client, error) {
	kubeClient, err := f.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes client: %v", err)
	}
	restConfig, err := f.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("get restConfig: %v", err)
	}
	crdClient, err := versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("get crd client: %v", err)
	}
	c7nClient, err := choerodon_version.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("get c7n client: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("error building choerodon clientset: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("error building c7n clientset: %v", err)
	}
	return &client{
		Factory:   f,
		client:    kubeClient,
		crdClient: crdClient,
		c7nClient: c7nClient,
	}, nil
}

func (c *client) GetRESTConfig() (*rest.Config, error) {
	return c.ToRESTConfig()
}

func (c *client) DeleteJob(namespace string, name string) error {
	job, err := c.client.BatchV1().Jobs(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return err
	}
	selector, err := meta_v1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return err
	}
	err = c.client.BatchV1().Jobs(namespace).Delete(name, &meta_v1.DeleteOptions{})
	if err != nil {
		return err
	}
	return c.client.CoreV1().Pods(namespace).DeleteCollection(&meta_v1.DeleteOptions{}, meta_v1.ListOptions{
		LabelSelector: selector.String(),
	})
}

func (c *client) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return c.client.Discovery(), nil
}

func (c *client) GetKubeClient() *kubernetes.Clientset {
	return c.client
}

func (c *client) GetCrdClient() *versioned.Clientset {
	return c.crdClient
}

func (c *client) GetC7nClient() *choerodon_version.Clientset {
	return c.c7nClient
}

func (c *client) BuildUnstructured(namespace string, manifest string) (Result, error) {
	var result Result

	result, err := c.NewBuilder().
		Unstructured().
		ContinueOnError().
		NamespaceParam(namespace).
		DefaultNamespace().
		Stream(bytes.NewBufferString(manifest), "").
		Flatten().
		Do().Infos()
	return result, err
}

func (c *client) GetC7NHelmRelease(name, namespace string) *v1alpha1.C7NHelmRelease {
	c7nHelmRelease, err := c.c7nClient.C7NHelmReleaseV1alpha1().C7nHelmReleases(namespace).Get(name, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		glog.Infof("C7nHelmRelease:%s not exists", name)
		return nil
	}
	return c7nHelmRelease
}

func (c *client) LogsForJob(namespace string, name string, jobLabel string) (string, string, error) {

	var jobStatus = ""
	job, err := c.client.BatchV1().Jobs(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return "", jobStatus, err
	}
	selector, err := meta_v1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", jobStatus, err
	}
	podList, err := c.client.CoreV1().Pods(namespace).List(meta_v1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", jobStatus, err
	}
	if len(podList.Items) == 0 {
		return "", jobStatus, nil
	}
	pod := podList.Items[0]

	//测试应用类型为2个容器的,返回指定容器的日志，job状态由指定容器的状态决定
	var options = &core_v1.PodLogOptions{}
	if jobLabel == model.TestLabel {
		for _, container := range pod.Spec.Containers {
			if strings.Contains(container.Name, testContainer) {
				options = &core_v1.PodLogOptions{
					Container: container.Name,
				}
			}
		}
	}

	if jobLabel == model.TestLabel {
		if len(pod.Status.ContainerStatuses) == 2 {
			for _, podStatus := range pod.Status.ContainerStatuses {
				if strings.Contains(podStatus.Name, testContainer) && podStatus.State.Terminated != nil && podStatus.State.Terminated.Reason == "Completed" {
					jobStatus = "success"
				}
			}
		}
	}

	req := c.client.CoreV1().Pods(namespace).GetLogs(pod.Name, options)
	readCloser, err := req.Stream()
	if err != nil {
		return "", jobStatus, err
	}
	defer readCloser.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, readCloser)
	return buf.String(), jobStatus, nil
}

func (c *client) CreateOrUpdateService(namespace string, serviceStr string) (*core_v1.Service, error) {
	svc := &core_v1.Service{}
	err := json.Unmarshal([]byte(serviceStr), svc)
	if err != nil {
		return nil, err
	}
	oldService, err := c.client.CoreV1().Services(namespace).Get(svc.Name, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.client.CoreV1().Services(namespace).Create(svc)
		}
		return nil, err
	}
	svc.ResourceVersion = oldService.ResourceVersion
	svc.Spec.ClusterIP = oldService.Spec.ClusterIP
	return c.client.CoreV1().Services(namespace).Update(svc)
}

func (c *client) GetService(namespace string, serviceName string) (string, error) {

	service, err := c.client.CoreV1().Services(namespace).Get(serviceName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if service.Annotations != nil && service.Annotations[model.CommitLabel] != "" {
		return service.Annotations[model.CommitLabel], nil
	}
	return "", nil
}

func (c *client) GetIngress(namespace string, ingressName string) (string, error) {
	ingress, err := c.client.ExtensionsV1beta1().Ingresses(namespace).Get(ingressName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if ingress.Annotations != nil && ingress.Annotations[model.CommitLabel] != "" {
		return ingress.Annotations[model.CommitLabel], nil
	}
	return "", nil
}

func (c *client) GetSecret(namespace string, secretName string) (string, error) {
	secret, err := c.client.CoreV1().Secrets(namespace).Get(secretName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if secret.Annotations != nil && secret.Annotations[model.CommitLabel] != "" {
		return secret.Annotations[model.CommitLabel], nil
	}
	return "", nil
}

func (c *client) GetConfigMap(namespace string, configMapName string) (string, error) {
	configMap, err := c.client.CoreV1().ConfigMaps(namespace).Get(configMapName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if configMap.Annotations != nil && configMap.Annotations[model.CommitLabel] != "" {
		return configMap.Annotations[model.CommitLabel], nil
	}
	return "", nil
}

func (c *client) GetPersistentVolumeClaim(namespace string, pvcName string) (string, string, error) {
	pvc, err := c.client.CoreV1().PersistentVolumeClaims(namespace).Get(pvcName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", "", err
		}
		return "", "", nil
	}
	if pvc.Annotations != nil && pvc.Annotations[model.CommitLabel] != "" {
		return pvc.Annotations[model.CommitLabel], string(pvc.Status.Phase), nil
	}
	return "", "", nil
}
func (c *client) GetPersistentVolume(namespace string, pvName string) (string, string, error) {
	pv, err := c.client.CoreV1().PersistentVolumes().Get(pvName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", "", err
		}
		return "", "", nil
	}
	if pv.Annotations != nil && pv.Annotations[model.CommitLabel] != "" {
		return pv.Annotations[model.CommitLabel], string(pv.Status.Phase), nil
	}
	return "", "", nil
}

func (c *client) IsReleaseJobRun(namespace, releaseName string) bool {
	labelMap := make(map[string]string)
	labelMap[model.ReleaseLabel] = releaseName
	options := &meta_v1.LabelSelector{
		MatchLabels: labelMap}

	slector, err := meta_v1.LabelSelectorAsSelector(options)
	if err != nil {
		glog.Infof("resource sync list job error: %v", err)
		return false
	}
	selector := meta_v1.ListOptions{
		LabelSelector: slector.String(),
	}

	jobList, err := c.client.BatchV1().Jobs(namespace).List(selector)

	if err != nil {
		glog.Infof("resource sync list job error: %v", err)
		return false
	}
	if len(jobList.Items) > 0 {
		return true
	}
	return false
}

func (c *client) GetNamespace(namespace string) error {
	_, err := c.client.CoreV1().Namespaces().Get(namespace, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("get Namespace error : %v", err)
		} else {
			return nil
		}
	}
	return nil
}

func (c *client) DeleteNamespace(namespace string) error {
	err := c.client.CoreV1().Namespaces().Delete(namespace, &meta_v1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("delete namespace error : %v", err)
		}
	}
	return nil
}

func (c *client) CreateOrUpdateIngress(namespace string, ingressStr string) (*ext_v1beta1.Ingress, error) {
	client, err := c.KubernetesClientSet()
	if err != nil {
		return nil, err
	}
	ing := &ext_v1beta1.Ingress{}
	err = json.Unmarshal([]byte(ingressStr), ing)
	if err != nil {
		return nil, err
	}
	if _, err := client.ExtensionsV1beta1().Ingresses(namespace).Get(ing.Name, meta_v1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return client.ExtensionsV1beta1().Ingresses(namespace).Create(ing)
		}
		return nil, err
	}
	return client.ExtensionsV1beta1().Ingresses(namespace).Update(ing)
}

func (c *client) DeleteService(namespace string, name string) error {
	client, err := c.KubernetesClientSet()
	if err != nil {
		return err
	}
	return client.CoreV1().Services(namespace).Delete(name, &meta_v1.DeleteOptions{})
}

func (c *client) DeleteIngress(namespace string, name string) error {
	client, err := c.KubernetesClientSet()
	if err != nil {
		return err
	}
	return client.ExtensionsV1beta1().Ingresses(namespace).Delete(name, &meta_v1.DeleteOptions{})
}

func (c *client) GetLogs(namespace string, pod string, containerName string) (io.ReadCloser, error) {
	var tailLinesDefault int64 = 1000
	req := c.client.CoreV1().Pods(namespace).GetLogs(
		pod,
		&core_v1.PodLogOptions{
			Follow:    true,
			Container: containerName,
			TailLines: &tailLinesDefault,
		},
	)
	readCloser, err := req.Stream()
	if err != nil {
		return nil, err
	}
	return readCloser, nil
}

func (c *client) Exec(namespace string, podName string, containerName string, local io.ReadWriter) error {
	config, err := c.ToRESTConfig()
	if err != nil {
		return err
	}
	pod, err := c.client.CoreV1().Pods(namespace).Get(podName, meta_v1.GetOptions{})
	if err != nil {
		glog.Errorf("can not find pod %s :%v", podName, err)
		return err
	}
	if pod.Status.Phase == core_v1.PodSucceeded || pod.Status.Phase == core_v1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}

	validShells := []string{"bash", "sh", "powershell", "cmd"}
	for _, testShell := range validShells {
		cmd := []string{testShell}
		req := c.client.CoreV1().RESTClient().Post().
			Resource("pods").
			Name(podName).
			Namespace(namespace).SubResource("exec").
			Param("container", containerName)
		req.VersionedParams(&core_v1.PodExecOptions{
			Stdin:     true,
			Stdout:    true,
			Stderr:    true,
			TTY:       true,
			Container: containerName,
			Command:   cmd,
		}, scheme.ParameterCodec)

		exec, err := remotecommand.NewSPDYExecutor(config, http.MethodPost, req.URL())
		if err == nil {
			err = exec.Stream(remotecommand.StreamOptions{
				Stdin:  local,
				Stdout: local,
				Stderr: local,
				Tty:    true,
			})
			if err == nil {
				return nil
			}

		}
	}
	glog.Errorf("no support command")
	return nil
}

func (c *client) GetSelectRelationPod(info *resource.Info, objPods map[string][]core_v1.Pod) (map[string][]core_v1.Pod, error) {
	if info == nil {
		return objPods, nil
	}

	selector, ok := getSelectorFromObject(info.Object)
	if !ok {
		return objPods, nil
	}

	pods, err := c.client.CoreV1().Pods(info.Namespace).List(meta_v1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: labels.Set(selector).AsSelector().String(),
	})
	if err != nil {
		return objPods, err
	}

	for _, pod := range pods.Items {
		if pod.APIVersion == "" {
			pod.APIVersion = "v1"
		}

		if pod.Kind == "" {
			pod.Kind = "Pod"
		}
		vk := pod.GroupVersionKind().Version + "/" + pod.GroupVersionKind().Kind

		if !isFoundPod(objPods[vk], pod) {
			objPods[vk] = append(objPods[vk], pod)
		}
	}
	return objPods, nil
}

func (c *client) LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error) {

	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		// add object and pod template label
		obj, err := labelRepoObj(info, namespace, version, c)

		if err != nil {
			return nil, fmt.Errorf("label object: %v", err)
		}
		if obj == nil {
			return nil, nil
		}

		objB, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %v", err)
		}

		m := make(map[string]interface{})
		err = yaml.Unmarshal(objB, &m)

		if err != nil {
			return nil, fmt.Errorf("yaml unmarshal: %v", err)
		}
		metaData := m["metadata"]
		metaDataMap := metaData.(map[string]interface{})

		annotationsMap := metaDataMap["annotations"]

		annotations := make(map[string]interface{})

		if annotationsMap == nil {
			annotationsMap = annotations
		} else {
			annotations = annotationsMap.(map[string]interface{})
		}
		annotations[model.CommitLabel] = commit
		metaDataMap["annotations"] = annotationsMap
		m["metadata"] = metaDataMap
		newObj, err := yaml.Marshal(m)

		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %v", err)
		}

		newManifestBuf.WriteString("\n---\n")
		newManifestBuf.Write(newObj)
	}

	return newManifestBuf, nil
}

// 给资源加标签
func labelRepoObj(info *resource.Info, namespace, version string, c *client) (runtime.Object, error) {

	obj := info.Object.(*unstructured.Unstructured)

	l := obj.GetLabels()
	annotation := obj.GetAnnotations()

	if l == nil {
		l = make(map[string]string)
	}

	if annotation == nil {
		annotation = make(map[string]string)
	}

	defer obj.SetLabels(l)
	defer obj.SetAnnotations(annotation)

	switch info.Mapping.GroupVersionKind.Kind {
	case "Service":
		l[model.NetworkLabel] = "service"
	case "Ingress":
		l[model.NetworkLabel] = "ingress"
	case "ConfigMap", "Secret":
	case "C7NHelmRelease":
		if namespace == "choerodon" {
			l[model.C7NHelmReleaseClusterLabel] = strconv.Itoa(int(ClusterId))
		}
		// 从集群中查出C7NHelmRelease，如果资源不存在，添加label["choerodon.io/C7NHelmRelease-status"]="INSTALL"，即安装操作
		// 如果资源存在，判断已更新的spec和集群存在的spec是否相同，相同添加label["choerodon.io/C7NHelmRelease-status"]="COMPLETE",即不会进行升级操作
		// 不同添加label["choerodon.io/C7NHelmRelease-status"]="UPGRADE",即进行升级操作
		c7nHelmRelease := c.GetC7NHelmRelease(obj.GetName(), namespace)
		newSpecMap := getSpecField(obj.Object)

		if c7nHelmRelease == nil {
			annotation[model.C7NHelmReleaseOperateAnnotation] = model.INSTALL
		} else {
			oldSpec := v1alpha1.C7NHelmReleaseSpec{}
			newSpec := v1alpha1.C7NHelmReleaseSpec{}

			// 获得集群中C7nHelmRelease的spec
			oldSpec = c7nHelmRelease.Spec

			// 获得更新后的spec
			newSpecJsonByte, err := json.Marshal(newSpecMap)
			if err != nil {
				glog.Info(err)
				break
			}
			if err := json.Unmarshal(newSpecJsonByte, &newSpec); err != nil {
				glog.Info(err)
				break
			}

			if !reflect.DeepEqual(newSpec, oldSpec) {
				annotation[model.C7NHelmReleaseOperateAnnotation] = model.UPGRADE
			}
		}
	case "PersistentVolumeClaim":
		l[model.PvcLabel] = fmt.Sprintf(model.PvcLabelValueFormat, ClusterId)
		l[model.NameLabel] = obj.GetName()
	case "PersistentVolume":
		l[model.EnvLabel] = namespace
		l[model.PvLabel] = fmt.Sprintf(model.PvLabelValueFormat, ClusterId)
		l[model.NameLabel] = obj.GetName()
	default:
		glog.Warningf("not support add label for object : %v", obj)
		return obj, nil
	}
	l[model.AgentVersionLabel] = AgentVersion

	return obj, nil
}

func nestedLocalObjectReferences(obj map[string]interface{}, fields ...string) ([]core_v1.LocalObjectReference, bool, error) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	if !found || err != nil {
		return nil, found, err
	}

	m, ok := val.([]core_v1.LocalObjectReference)
	if ok {
		return m, true, nil
		//return nil, false, fmt.Errorf("%v accessor error: %v is of the type %T, expected []core_v1.LocalObjectReference", strings.Join(fields, "."), val, val)
	}

	if m, ok := val.([]interface{}); ok {
		secrets := make([]core_v1.LocalObjectReference, 0)
		for _, v := range m {
			if v1, ok := v.(map[string]interface{}); ok {
				v2 := v1["name"]
				secret := core_v1.LocalObjectReference{}
				if secret.Name, ok = v2.(string); ok {
					secrets = append(secrets, secret)
				}
			}
		}
		return secrets, true, nil
	}
	return m, true, nil
}

func getSpecField(obj map[string]interface{}) map[string]interface{} {
	specFields, _, err := unstructured.NestedMap(obj, "spec")
	if err != nil {
		glog.Warningf("Get Template Labels failed, %v", err)
	}
	if specFields == nil {
		specFields = make(map[string]interface{})
	}
	return specFields
}

func getTemplateLabels(obj map[string]interface{}) map[string]string {
	tplLabels, _, err := unstructured.NestedStringMap(obj, "spec", "template", "metadata", "labels")
	if err != nil {
		glog.Warningf("Get Template Labels failed, %v", err)
	}
	if tplLabels == nil {
		tplLabels = make(map[string]string)
	}
	return tplLabels
}

func setTemplateLabels(obj map[string]interface{}, templateLabels map[string]string) error {
	return unstructured.SetNestedStringMap(obj, templateLabels, "spec", "template", "metadata", "labels")
}

func setScale(obj map[string]interface{}, replicas int64) error {
	return unstructured.SetNestedField(obj, replicas, "spec", "replicas")
}

func isFoundPod(podItem []core_v1.Pod, pod core_v1.Pod) bool {
	for _, value := range podItem {
		if (value.Namespace == pod.Namespace) && (value.Name == pod.Name) {
			return true
		}
	}
	return false
}

func getSelectorFromObject(obj runtime.Object) (map[string]string, bool) {
	kind := obj.GetObjectKind().GroupVersionKind().Kind
	t := obj.(*unstructured.Unstructured)
	if kind == "ReplicationController" {
		matchLabels, result, err := unstructured.NestedStringMap(t.Object, "spec", "selector")
		if result == false && err != nil {
			return nil, false
		}
		return matchLabels, true
	} else {
		matchLabels, result, err := unstructured.NestedStringMap(t.Object, "spec", "selector", "matchLabels")
		if result == false && err != nil {
			return nil, false
		}
		return matchLabels, true
	}
}

func (c *client) GetPodByLabelSelector(info *resource.Info) (*v1.PodList, error) {

	selector, ok := getSelectorFromObject(info.Object)
	if !ok {
		return nil, nil
	}

	pods, err := c.client.CoreV1().Pods(info.Namespace).List(meta_v1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: labels.Set(selector).AsSelector().String(),
	})
	if err != nil {
		return nil, err
	}
	return pods, err
}

func (c *client) CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error) {
	_, err := c.client.CoreV1().Secrets(namespace).Get(secret.Name, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.client.CoreV1().Secrets(namespace).Create(secret)
		}
		return nil, err
	}
	return c.client.CoreV1().Secrets(namespace).Update(secret)
}

func InArray(expectedResourceKind []string, kind string) bool {
	for _, item := range expectedResourceKind {
		if item == kind {
			return true
		}
	}
	return false
}
