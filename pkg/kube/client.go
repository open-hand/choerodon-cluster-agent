package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"io"
	appsv1 "k8s.io/api/apps/v1"
	appsv1beta1 "k8s.io/api/apps/v1beta1"
	appsv1beta2 "k8s.io/api/apps/v1beta2"
	batch "k8s.io/api/batch/v1"
	core_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/cli-runtime/pkg/genericclioptions/resource"
	"k8s.io/client-go/discovery"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"net/http"
	"strconv"
	"strings"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
)

var ClusterId int32

type Client interface {
	DeleteJob(namespace string, name string) error
	LogsForJob(namespace string, name string, jobLabel string) (string, string, error)
	CreateOrUpdateService(namespace string, serviceStr string) (*core_v1.Service, error)
	CreateOrUpdateIngress(namespace string, ingressStr string) (*ext_v1beta1.Ingress, error)
	//todo:remove
	GetClientSet() (internalclientset.Interface, error)
	//---
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
	DeleteService(namespace string, name string) error
	DeleteIngress(namespace string, name string) error
	StartResources(namespace string, manifest string) error
	StopResources(namespace string, manifest string) error
	GetLogs(namespace string, pod string, container string) (io.ReadCloser, error)
	Exec(namespace string, podName string, containerName string, local io.ReadWriter) error
	LabelObjects(namespace string, command int, imagePullSecret []core_v1.LocalObjectReference, manifest string, releaseName string, app string, version string) (*bytes.Buffer, error)
	LabelTestObjects(namespace string, imagePullSecret []core_v1.LocalObjectReference, manifest string, releaseName string, app string, version string, label string) (*bytes.Buffer, error)
	LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error)
	GetService(namespace string, serviceName string) (string, error)
	GetIngress(namespace string, ingressName string) (string, error)
	GetPersistentVolumeClaim(namespace string, pvcName string) (string, string, error)
	GetPersistentVolume(namespace string, pvcName string) (string, string, error)
	GetNamespace(namespace string) error
	DeleteNamespace(namespace string) error
	GetSecret(namespace string, secretName string) (string, error)
	GetKubeClient() *kubernetes.Clientset
	GetRESTConfig() (*rest.Config, error)
	IsReleaseJobRun(namespace, releaseName string) bool
	CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error)
	BuildUnstructured(namespace string, manifest string) (Result, error)
	//todo: delete follow func
	GetSelectRelationPod(info *resource.Info, objPods map[string][]core_v1.Pod) (map[string][]core_v1.Pod, error)
}

const testContainer string = "automation-test"

var AgentVersion string

type client struct {
	cmdutil.Factory
	client *kubernetes.Clientset
}

func NewClient(f cmdutil.Factory) (Client, error) {
	kubeClient, err := f.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes client: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("error building choerodon clientset: %v", err)
	}
	if err != nil {
		return nil, fmt.Errorf("error building c7n clientset: %v", err)
	}
	return &client{
		Factory: f,
		client:  kubeClient,
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

//todo:remove
func (c *client) GetClientSet() (internalclientset.Interface, error) {
	return nil, nil
}

func (c *client) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return c.client.Discovery(), nil
}

func (c *client) GetKubeClient() *kubernetes.Clientset {
	return c.client
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

// AsVersionedObject converts a runtime.object to a versioned object.
func (c *client) AsVersionedObject(obj runtime.Object) (runtime.Object, error) {
	json, err := runtime.Encode(unstructured.UnstructuredJSONScheme, obj)
	if err != nil {
		return nil, err
	}
	decoder := scheme.Codecs.UniversalDecoder()
	versions := &runtime.VersionedObjects{}
	err = runtime.DecodeInto(decoder, json, versions)
	return versions.First(), err
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

func (c *client) StopResources(namespace string, manifest string) error {
	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return fmt.Errorf("build unstructured: %v", err)
	}

	clientSet := c.GetKubeClient()

	for _, info := range result {
		s, err := clientSet.AppsV1().Deployments(info.Namespace).GetScale(info.Name, meta_v1.GetOptions{})
		if err != nil {
			return err
		}
		s.Spec.Replicas = 0

		_, err = clientSet.AppsV1().Deployments(info.Namespace).UpdateScale(info.Name, s)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) StartResources(namespace string, manifest string) error {
	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return fmt.Errorf("build unstructured: %v", err)
	}
	for _, info := range result {
		_, err := resource.NewHelper(info.Client, info.Mapping).Replace(info.Namespace, info.Name, true, info.Object)
		if err != nil {
			glog.V(2).Infof("replace: %v", err)
		}
	}
	return nil
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
		}, legacyscheme.ParameterCodec)

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

	versioned, err := c.AsVersionedObject(info.Object)
	switch {
	case runtime.IsNotRegisteredError(err):
		return objPods, nil
	case err != nil:
		return objPods, err
	}

	selector, ok := getSelectorFromObject(versioned)
	if !ok {
		return objPods, nil
	}

	pods, err := c.client.Core().Pods(info.Namespace).List(meta_v1.ListOptions{
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

func (c *client)LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error) {

	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		// add object and pod template label
		obj, err := labelRepoObj(info, version)

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

func (c *client) LabelObjects(namespace string,
	command int,
	imagePullSecret []core_v1.LocalObjectReference,
	manifest string,
	releaseName string,
	app string,
	version string) (*bytes.Buffer, error) {
	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		// add object and pod template label
		obj, err := labelObject(imagePullSecret, command, info, releaseName, app, version)
		if err != nil {
			return nil, fmt.Errorf("label object: %v", err)
		}

		objB, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %v", err)
		}
		newManifestBuf.WriteString("\n---\n")
		newManifestBuf.Write(objB)
	}

	return newManifestBuf, nil
}

// todo: what this do for ??
func labelRepoObj(info *resource.Info, version string) (runtime.Object, error) {

	obj := info.Object.(*unstructured.Unstructured)

	l := obj.GetLabels()

	if l == nil {
		l = make(map[string]string)
	}
	defer obj.SetLabels(l)

	switch info.Mapping.GroupVersionKind.Kind {
	case "Service":
		l[model.NetworkLabel] = "service"
	case "Ingress":
		l[model.NetworkLabel] = "ingress"
	case "ConfigMap", "Secret":
	case "C7NHelmRelease":
		if info.Namespace=="choerodon" {
			glog.Info("prometheus-0.10.0-0af10")
		}
	case "PersistentVolumeClaim":
		l[model.PvcLabel] = fmt.Sprintf(model.PvcLabelValueFormat,ClusterId)
	case "PersistentVolume":
		l[model.PvLabel] = fmt.Sprintf(model.PvLabelValueFormat, ClusterId)
	default:
		glog.Warningf("not support add label for object : %v", obj)
		return obj, nil
	}
	l[model.AgentVersionLabel] = AgentVersion

	return obj, nil
}

func nestedLocalObjectReferences(obj map[string]interface{}, fields ...string) ([]core_v1.LocalObjectReference, bool, error) {
	val, found, err := unstructured.NestedFieldNoCopy(obj, fields...)
	fmt.Println(val)
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

// Provide a common method for adding labels
func addLabel(imagePullSecret []core_v1.LocalObjectReference,
	command int,
	info *resource.Info, version, releaseName, app, testLabel string, isTest bool) (runtime.Object, error) {
	t := info.Object.(*unstructured.Unstructured)

	l := t.GetLabels()
	defer t.SetLabels(l)

	if l == nil {
		l = make(map[string]string)
	}

	var addBaseLabels = func() {
		l[model.ReleaseLabel] = releaseName
		l[model.AgentVersionLabel] = AgentVersion
	}
	var addAppLabels = func() {
		l[model.AppLabel] = app
		l[model.AppVersionLabel] = version
	}

	var addTemplateAppLabels = func() {
		tplLabels := getTemplateLabels(t.Object)
		tplLabels[model.ReleaseLabel] = releaseName
		tplLabels[model.AgentVersionLabel] = AgentVersion
		if !isTest {
			tplLabels[model.CommandLabel] = strconv.Itoa(command)
		}
		tplLabels[model.AppLabel] = app
		tplLabels[model.AppVersionLabel] = version
		if err := setTemplateLabels(t.Object, tplLabels); err != nil {
			glog.Warningf("Set Template Labels failed, %v", err)
		}
	}
	var addSelectorAppLabels = func() {
		selectorLabels, _, err := unstructured.NestedStringMap(t.Object, "spec", "selector", "matchLabels")
		if err != nil {
			glog.Warningf("Get Selector Labels failed, %v", err)
		}
		if selectorLabels == nil {
			selectorLabels = make(map[string]string)
		}
		selectorLabels[model.ReleaseLabel] = releaseName
		if err := unstructured.SetNestedStringMap(t.Object, selectorLabels, "spec", "selector", "matchLabels"); err != nil {
			glog.Warningf("Set Selector label failed, %v", err)
		}
	}

	// add private image pull secrets
	var addImagePullSecrets = func() {
		secrets, _, err := nestedLocalObjectReferences(t.Object, "spec", "template", "spec", "imagePullSecrets")
		if err != nil {
			glog.Warningf("Get ImagePullSecrets failed, %v", err)
		}
		if secrets == nil {
			secrets = make([]core_v1.LocalObjectReference, 0)

		}
		secrets = append(secrets, imagePullSecret...)
		// SetNestedField method just support a few types
		s := make([]interface{}, 0)
		for _, secret := range secrets {
			m := make(map[string]interface{})
			m["name"] = secret.Name
			s = append(s, m)
		}
		if err := unstructured.SetNestedField(t.Object, s, "spec", "template", "spec", "imagePullSecrets"); err != nil {
			glog.Warningf("Set ImagePullSecrets failed, %v", err)
		}
	}

	kind := info.Mapping.GroupVersionKind.Kind
	switch kind {
	case "ReplicationController", "ReplicaSet", "Deployment":
		addAppLabels()
		addTemplateAppLabels()
		addSelectorAppLabels()
		addImagePullSecrets()
	case "ConfigMap":
	case "Service":
		l[model.NetworkLabel] = "service"
		l[model.NetworkNoDelLabel] = "true"
	case "Ingress":
		l[model.NetworkLabel] = "ingress"
		l[model.NetworkNoDelLabel] = "true"
	case "Job":
		addImagePullSecrets()
		if isTest {
			l[model.TestLabel] = testLabel
			tplLabels := getTemplateLabels(t.Object)
			tplLabels[model.TestLabel] = testLabel
			tplLabels[model.ReleaseLabel] = releaseName
			if err := setTemplateLabels(t.Object, tplLabels); err != nil {
				glog.Warningf("Set Test-Template Labels failed, %v", err)
			}
		}
	case "DaemonSet", "StatefulSet":
		addAppLabels()
		addTemplateAppLabels()
		addImagePullSecrets()
	case "Secret":
		addAppLabels()
	case "RoleBinding", "ClusterRoleBinding", "Role", "ClusterRole", "PodSecurityPolicy", "ServiceAccount":
	case "Pod":
		addAppLabels()
	default:
		glog.Warningf("Add Choerodon label failed, unsupported object: Kind %s of Release %s", kind, releaseName)
		return t, nil
	}
	// add base labels
	addBaseLabels()
	return t, nil
}

func labelObject(imagePullSecret []core_v1.LocalObjectReference,
	command int,
	info *resource.Info,
	releaseName string,
	app string,
	version string) (runtime.Object, error) {

	return addLabel(imagePullSecret, command, info, version, releaseName, app, "", false)
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
	switch typed := obj.(type) {

	case *core_v1.ReplicationController:
		return typed.Spec.Selector, true

	case *ext_v1beta1.ReplicaSet:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1.ReplicaSet:
		return typed.Spec.Selector.MatchLabels, true

	case *ext_v1beta1.Deployment:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1beta1.Deployment:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1beta2.Deployment:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1.Deployment:
		return typed.Spec.Selector.MatchLabels, true

	case *ext_v1beta1.DaemonSet:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1beta2.DaemonSet:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1.DaemonSet:
		return typed.Spec.Selector.MatchLabels, true

	case *batch.Job:
		return typed.Spec.Selector.MatchLabels, true

	case *appsv1beta1.StatefulSet:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1beta2.StatefulSet:
		return typed.Spec.Selector.MatchLabels, true
	case *appsv1.StatefulSet:
		return typed.Spec.Selector.MatchLabels, true

	default:
		return nil, false
	}
}

func (c *client) LabelTestObjects(namespace string,
	imagePullSecret []core_v1.LocalObjectReference,
	manifest string,
	releaseName string,
	app string,
	version string,
	label string) (*bytes.Buffer, error) {
	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		// add object and pod template label
		obj, err := labelTestObject(info, imagePullSecret, releaseName, app, version, label)
		if err != nil {
			return nil, fmt.Errorf("label object: %v", err)
		}

		objB, err := yaml.Marshal(obj)
		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %v", err)
		}
		newManifestBuf.WriteString("\n---\n")
		newManifestBuf.Write(objB)
	}

	return newManifestBuf, nil
}

func labelTestObject(info *resource.Info,
	imagePullSecret []core_v1.LocalObjectReference,
	releaseName string,
	app string,
	version string,
	label string) (runtime.Object, error) {

	return addLabel(imagePullSecret, 0, info, version, releaseName, app, label, true)
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
