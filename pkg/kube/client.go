package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	chrclientset "github.com/choerodon/choerodon-cluster-agent/pkg/client/clientset/versioned"
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
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/remotecommand"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"
	"net/http"
	"strings"

	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	model_helm "github.com/choerodon/choerodon-cluster-agent/pkg/model/helm"
)

type Client interface {
	DeleteJob(namespace string, name string) error
	LogsForJob(namespace string, name string, jobLabel string) (string, string, error)
	GetResources(namespace string, manifest string) ([]*model_helm.ReleaseResource, error)
	CreateOrUpdateService(namespace string, serviceStr string) (*core_v1.Service, error)
	CreateOrUpdateIngress(namespace string, ingressStr string) (*ext_v1beta1.Ingress, error)
	GetClientSet() (internalclientset.Interface, error)
	DeleteService(namespace string, name string) error
	DeleteIngress(namespace string, name string) error
	StartResources(namespace string, manifest string) error
	StopResources(namespace string, manifest string) error
	GetLogs(namespace string, pod string, container string) (io.ReadCloser, error)
	Exec(namespace string, podName string, containerName string, local io.ReadWriter) error
	LabelObjects(namespace string, imagePullSecret []core_v1.LocalObjectReference, manifest string, releaseName string, app string, version string) (*bytes.Buffer, error)
	LabelTestObjects(namespace string, imagePullSecret []core_v1.LocalObjectReference, manifest string, releaseName string, app string, version string, label string) (*bytes.Buffer, error)
	LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error)
	GetService(namespace string, serviceName string) (string, error)
	GetIngress(namespace string, ingressName string) (string, error)
	GetNamespace(namespace string) error
	DeleteNamespace(namespace string) error
	GetSecret(namespace string, secretName string) (string, error)
	GetC7nHelmRelease(namespace string, releaseName string) (*v1alpha1.C7NHelmRelease, error)
	GetKubeClient() (*kubernetes.Clientset)
	GetC7NClient() (*chrclientset.Clientset)
	IsReleaseJobRun(namespace, releaseName string) bool
	CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error)
}

const testContainer string = "automation-test"

var AgentVersion string

type client struct {
	cmdutil.Factory
	client    *kubernetes.Clientset
	c7nClient *chrclientset.Clientset
}

func NewClient(f cmdutil.Factory) (Client, error) {
	kubeClient, err := f.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes client: %v", err)
	}
	clientConfig, err := f.ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("error building choerodon clientset: %v", err)
	}
	c7nClient, err := chrclientset.NewForConfig(clientConfig)
	if err != nil {
		return nil, fmt.Errorf("error building c7n clientset: %v", err)
	}
	return &client{
		Factory:   f,
		client:    kubeClient,
		c7nClient: c7nClient,
	}, nil
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

func (c *client) GetClientSet() (internalclientset.Interface, error) {
	return c.ClientSet()
}

func (c *client) GetKubeClient() (*kubernetes.Clientset) {
	return c.client
}

func (c *client) GetC7NClient() (*chrclientset.Clientset) {
	return c.c7nClient
}

func (c *client) GetResources(namespace string, manifest string) ([]*model_helm.ReleaseResource, error) {
	resources := make([]*model_helm.ReleaseResource, 0, 10)
	result, err := c.buildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	var objPods = make(map[string][]core_v1.Pod)
	for _, info := range result {
		if err := info.Get(); err != nil {
			continue
		}
		hrr := &model_helm.ReleaseResource{
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
		objPods, err = c.getSelectRelationPod(info, objPods)
		//here, we will add the objPods to the objs
		for _, podItems := range objPods {
			for i := range podItems {
				hrr := &model_helm.ReleaseResource{
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

func (c *client) buildUnstructured(namespace string, manifest string) (Result, error) {
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
	versions := &runtime.VersionedObjects{}
	err = runtime.DecodeInto(c.Decoder(true), json, versions)
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

func (c *client) GetC7nHelmRelease(namespace string, releaseName string) (*v1alpha1.C7NHelmRelease, error) {
	release, err := c.c7nClient.ChoerodonV1alpha1().C7NHelmReleases(namespace).Get(releaseName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, err
		}
		return nil, nil
	}
	if release.Annotations != nil && release.Annotations[model.CommitLabel] != "" {
		return release, nil
	}
	return nil, nil
}

func (c *client) IsReleaseJobRun(namespace, releaseName string) bool {
	labelMap := make(map[string]string)
	labelMap[model.ReleaseLabel] = releaseName
	options := &meta_v1.LabelSelector{
		MatchLabels: labelMap,}

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
	result, err := c.buildUnstructured(namespace, manifest)
	if err != nil {
		return fmt.Errorf("build unstructured: %v", err)
	}
	for _, info := range result {
		mapping := info.ResourceMapping()
		scaler, err := c.Scaler(mapping)
		if err != nil {
			glog.V(2).Infof("get scaler: %v", err)
			continue
		}
		precondition := &kubectl.ScalePrecondition{Size: -1, ResourceVersion: ""}
		if _, err := scaler.ScaleSimple(info.Namespace, info.Name, precondition, 0); err != nil {
			glog.V(2).Infof("scale to 0: %v", err)
		}
	}
	return nil
}

func (c *client) StartResources(namespace string, manifest string) error {
	result, err := c.buildUnstructured(namespace, manifest)
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
	config, err := c.ClientConfig()
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

func (c *client) getSelectRelationPod(info *resource.Info, objPods map[string][]core_v1.Pod) (map[string][]core_v1.Pod, error) {
	if info == nil {
		return objPods, nil
	}
	versioned, err := info.Versioned()
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

func (c *client) LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error) {

	result, err := c.buildUnstructured(namespace, manifest)
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
	imagePullSecret []core_v1.LocalObjectReference,
	manifest string,
	releaseName string,
	app string,
	version string) (*bytes.Buffer, error) {
	result, err := c.buildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		// add object and pod template label
		obj, err := labelObject(imagePullSecret, info, releaseName, app, version)
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

func labelRepoObj(info *resource.Info, version string) (runtime.Object, error) {
	versioned, err := info.Versioned()
	switch {
	case runtime.IsNotRegisteredError(err):
		return nil, nil

	case err != nil:
		return nil, err
	}
	obj := versioned.DeepCopyObject()

	switch typed := obj.(type) {
	// ReplicationController
	// Deployment

	// Service
	case *core_v1.Service:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.NetworkLabel] = "service"
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		// Ingress
	case *ext_v1beta1.Ingress:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.NetworkLabel] = "ingress"
		typed.Labels[model.AgentVersionLabel] = AgentVersion
	case *core_v1.ConfigMap:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		//typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion
	case *core_v1.Secret:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.AgentVersionLabel] = AgentVersion

	default:
		glog.Warningf("label object not matched: %v", obj)
		return obj, nil
	}
	return obj, nil
}

func labelObject(imagePullSecret []core_v1.LocalObjectReference,
	info *resource.Info,
	releaseName string,
	app string,
	version string) (runtime.Object, error) {
	versioned, err := info.Versioned()
	switch {
	case runtime.IsNotRegisteredError(err):
		return nil, nil
	case err != nil:
		return nil, err
	}

	obj := versioned.DeepCopyObject()
	switch typed := obj.(type) {
	// ReplicationController
	case *core_v1.ReplicationController:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// ReplicaSet
	case *ext_v1beta1.ReplicaSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.ReplicaSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	// Deployment
	case *ext_v1beta1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	// ConfigMap
	case *core_v1.ConfigMap:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion

	// Service
	case *core_v1.Service:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.NetworkLabel] = "service"
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Labels[model.NetworkNoDelLabel] = "true"

	// Ingress
	case *ext_v1beta1.Ingress:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.NetworkLabel] = "ingress"
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Labels[model.NetworkNoDelLabel] = "true"

	// Job
	case *batch.Job:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	// DaemonSet
	case *ext_v1beta1.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	// StatefulSet
	case *appsv1beta1.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	// Secret
	case *core_v1.Secret:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
	default:
		glog.Warningf("label object not matched: %v", obj)
	}

	return obj, nil
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
	result, err := c.buildUnstructured(namespace, manifest)
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
	versioned, err := info.Versioned()
	switch {
	case runtime.IsNotRegisteredError(err):
		return nil, nil
	case err != nil:
		return nil, err
	}

	obj := versioned.DeepCopyObject()
	switch typed := obj.(type) {
	// ReplicationController
	case *core_v1.ReplicationController:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// ReplicaSet
	case *ext_v1beta1.ReplicaSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.ReplicaSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// Deployment
	case *ext_v1beta1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.Deployment:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		if typed.Spec.Selector == nil {
			typed.Spec.Selector = &meta_v1.LabelSelector{}
		}
		if typed.Spec.Selector.MatchLabels == nil {
			typed.Spec.Selector.MatchLabels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.AppVersionLabel] = version
		typed.Spec.Template.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.AppLabel] = app
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Spec.Selector.MatchLabels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// ConfigMap
	case *core_v1.ConfigMap:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion

		// Service
	case *core_v1.Service:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.NetworkLabel] = "service"
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Labels[model.NetworkNoDelLabel] = "true"

		// Ingress
	case *ext_v1beta1.Ingress:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.NetworkLabel] = "ingress"
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Labels[model.NetworkNoDelLabel] = "true"

		// Job
	case *batch.Job:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Labels[model.TestLabel] = label
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName
		typed.Spec.Template.Labels[model.TestLabel] = label

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// DaemonSet
	case *ext_v1beta1.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.DaemonSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// StatefulSet
	case *appsv1beta1.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1beta2.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

	case *appsv1.StatefulSet:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		if typed.Spec.Template.Labels == nil {
			typed.Spec.Template.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
		typed.Spec.Template.Labels[model.ReleaseLabel] = releaseName

		if typed.Spec.Template.Spec.ImagePullSecrets != nil {
			typed.Spec.Template.Spec.ImagePullSecrets = append(typed.Spec.Template.Spec.ImagePullSecrets, imagePullSecret...)
		} else {
			typed.Spec.Template.Spec.ImagePullSecrets = imagePullSecret
		}

		// Secret
	case *core_v1.Secret:
		if typed.Labels == nil {
			typed.Labels = make(map[string]string)
		}
		typed.Labels[model.ReleaseLabel] = releaseName
		typed.Labels[model.AppLabel] = app
		typed.Labels[model.AppVersionLabel] = version
		typed.Labels[model.AgentVersionLabel] = AgentVersion
	default:
		glog.Warningf("label object not matched: %v", obj)
	}

	return obj, nil
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
