package kube

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	v1_versioned "github.com/choerodon/choerodon-cluster-agent/pkg/apis/certificate/v1/client/clientset/versioned"
	choerodon_version "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/clientset/versioned"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/ghodss/yaml"
	"github.com/golang/glog"
	"io"
	core_v1 "k8s.io/api/core/v1"
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
	"strings"
)

type Client interface {
	DeleteJob(namespace string, name string) error
	LogsForJob(namespace string, name string, jobLabel string) (string, string, error)
	ConvertCertificate(namespace, manifest, commit string) (*bytes.Buffer, error, bool)
	CreateOrUpdateService(namespace string, serviceStr string) (*core_v1.Service, error)
	CreateOrUpdateIngress(namespace string, ingressStr string) (*ext_v1beta1.Ingress, error)
	GetDiscoveryClient() (discovery.DiscoveryInterface, error)
	DeleteService(namespace string, name string) error
	DeleteIngress(namespace string, name string) error
	GetLogs(namespace string, pod string, container string, follow bool, previous bool, line int64) (io.ReadCloser, error)
	Exec(namespace string, podName string, containerName string, local io.ReadWriter) error
	LabelRepoObj(namespace, manifest, version string, commit string) (*bytes.Buffer, error)
	GetService(namespace string, serviceName string) (string, error)
	GetIngress(namespace string, ingressName string) (string, error)
	GetConfigMap(namespace string, configMapName string) (string, error)
	GetPersistentVolumeClaim(namespace string, pvcName string) (string, string, error)
	GetPersistentVolume(namespace string, pvcName string) (string, string, error)
	GetDeployment(namespace string, deploymentName string) (string, error)
	GetStatefulSet(namespace string, statefulSetName string) (string, error)
	GetJob(namespace string, jobName string) (string, error)
	GetCronJob(namespace string, cronJobName string) (string, error)
	GetDaemonSet(namespace string, daemonSetName string) (string, error)
	GetNamespace(namespace string) error
	DeleteNamespace(namespace string) error
	GetSecret(namespace string, secretName string) (string, error)
	IsSecretExist(namespace string, secretName string) (bool, error)
	GetKubeClient() *kubernetes.Clientset
	GetV1CrdClient() *v1_versioned.Clientset
	//GetV1alpha1CrdClient() *v1alpha1_versioned.Clientset
	GetC7nClient() *choerodon_version.Clientset
	GetRESTConfig() (*rest.Config, error)
	IsReleaseJobRun(namespace, releaseName string) bool
	CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error)
	BuildUnstructured(namespace string, manifest string) (ResourceList, error)
	//todo: delete follow func
	GetSelectRelationPod(info *resource.Info) (map[string][]core_v1.Pod, error)
	GetC7NHelmRelease(name, namespace string) *v1alpha1.C7NHelmRelease
	UpdateC7nHelmRelease(release *v1alpha1.C7NHelmRelease, namespace string)
}

const testContainer string = "automation-test"

type client struct {
	cmdutil.Factory
	client      *kubernetes.Clientset
	v1CrdClient *v1_versioned.Clientset
	//v1alpha1CrdClient *v1alpha1_versioned.Clientset
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
	v1CrdClient, err := v1_versioned.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("get v1 crd client: %v", err)
	}
	//v1alpha1CrdClient, err := v1alpha1_versioned.NewForConfig(restConfig)
	//if err != nil {
	//	return nil, fmt.Errorf("get v1alpha1 crd client: %v", err)
	//}

	c7nClient, err := choerodon_version.NewForConfig(restConfig)
	if err != nil {
		return nil, fmt.Errorf("error building c7n clientset: %v", err)
	}
	return &client{
		Factory:     f,
		client:      kubeClient,
		v1CrdClient: v1CrdClient,
		//v1alpha1CrdClient: v1alpha1CrdClient,
		c7nClient: c7nClient,
	}, nil
}

func (c *client) GetRESTConfig() (*rest.Config, error) {
	return c.ToRESTConfig()
}

func (c *client) DeleteJob(namespace string, name string) error {
	job, err := c.client.BatchV1().Jobs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return err
	}
	selector, err := meta_v1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return err
	}
	err = c.client.BatchV1().Jobs(namespace).Delete(context.TODO(), name, meta_v1.DeleteOptions{})
	if err != nil {
		return err
	}
	return c.client.CoreV1().Pods(namespace).DeleteCollection(context.TODO(), meta_v1.DeleteOptions{}, meta_v1.ListOptions{
		LabelSelector: selector.String(),
	})
}

func (c *client) GetDiscoveryClient() (discovery.DiscoveryInterface, error) {
	return c.client.Discovery(), nil
}

func (c *client) GetKubeClient() *kubernetes.Clientset {
	return c.client
}

func (c *client) GetV1CrdClient() *v1_versioned.Clientset {
	return c.v1CrdClient
}

//func (c *client) GetV1alpha1CrdClient() *v1alpha1_versioned.Clientset {
//	return c.v1alpha1CrdClient
//}

func (c *client) GetC7nClient() *choerodon_version.Clientset {
	return c.c7nClient
}

func (c *client) BuildUnstructured(namespace string, manifest string) (ResourceList, error) {
	var result ResourceList

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
	c7nHelmRelease, err := c.c7nClient.C7NHelmReleaseV1alpha1().C7nHelmReleases(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if errors.IsNotFound(err) {
		glog.Infof("C7nHelmRelease:%s not exists", name)
		return nil
	}
	return c7nHelmRelease
}

func (c *client) UpdateC7nHelmRelease(release *v1alpha1.C7NHelmRelease, namespace string) {
	_, err := c.c7nClient.C7NHelmReleaseV1alpha1().C7nHelmReleases(namespace).Update(context.TODO(), release)
	if err != nil {
		glog.Infof("update C7NHelmRelease %s failed,err is:%s", release.GetName(), err.Error())
	}
}

func (c *client) LogsForJob(namespace string, name string, jobLabel string) (string, string, error) {

	var jobStatus = ""
	job, err := c.client.BatchV1().Jobs(namespace).Get(context.TODO(), name, meta_v1.GetOptions{})
	if err != nil {
		return "", jobStatus, err
	}
	selector, err := meta_v1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", jobStatus, err
	}
	podList, err := c.client.CoreV1().Pods(namespace).List(context.TODO(), meta_v1.ListOptions{LabelSelector: selector.String()})
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
	readCloser, err := req.Stream(context.TODO())
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
	oldService, err := c.client.CoreV1().Services(namespace).Get(context.TODO(), svc.Name, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.client.CoreV1().Services(namespace).Create(context.TODO(), svc, meta_v1.CreateOptions{})
		}
		return nil, err
	}
	svc.ResourceVersion = oldService.ResourceVersion
	svc.Spec.ClusterIP = oldService.Spec.ClusterIP
	return c.client.CoreV1().Services(namespace).Update(context.TODO(), svc, meta_v1.UpdateOptions{})
}

func (c *client) GetService(namespace string, serviceName string) (string, error) {

	service, err := c.client.CoreV1().Services(namespace).Get(context.TODO(), serviceName, meta_v1.GetOptions{})
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
	ingress, err := c.client.ExtensionsV1beta1().Ingresses(namespace).Get(context.TODO(), ingressName, meta_v1.GetOptions{})
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
	secret, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, meta_v1.GetOptions{})
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

func (c *client) IsSecretExist(namespace string, secretName string) (bool, error) {
	secret, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), secretName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return false, err
		}
		return false, nil
	}
	if secret.Type == "kubernetes.io/tls" {
		return true, nil
	} else {
		return false, nil
	}
}

func (c *client) GetConfigMap(namespace string, configMapName string) (string, error) {
	configMap, err := c.client.CoreV1().ConfigMaps(namespace).Get(context.TODO(), configMapName, meta_v1.GetOptions{})
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
	pvc, err := c.client.CoreV1().PersistentVolumeClaims(namespace).Get(context.TODO(), pvcName, meta_v1.GetOptions{})
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
	pv, err := c.client.CoreV1().PersistentVolumes().Get(context.TODO(), pvName, meta_v1.GetOptions{})
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

	jobList, err := c.client.BatchV1().Jobs(namespace).List(context.TODO(), selector)

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
	_, err := c.client.CoreV1().Namespaces().Get(context.TODO(), namespace, meta_v1.GetOptions{})
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
	err := c.client.CoreV1().Namespaces().Delete(context.TODO(), namespace, meta_v1.DeleteOptions{})
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
	if _, err := client.ExtensionsV1beta1().Ingresses(namespace).Get(context.TODO(), ing.Name, meta_v1.GetOptions{}); err != nil {
		if errors.IsNotFound(err) {
			return client.ExtensionsV1beta1().Ingresses(namespace).Create(context.TODO(), ing, meta_v1.CreateOptions{})
		}
		return nil, err
	}
	return client.ExtensionsV1beta1().Ingresses(namespace).Update(context.TODO(), ing, meta_v1.UpdateOptions{})
}

func (c *client) DeleteService(namespace string, name string) error {
	client, err := c.KubernetesClientSet()
	if err != nil {
		return err
	}
	return client.CoreV1().Services(namespace).Delete(context.TODO(), name, meta_v1.DeleteOptions{})
}

func (c *client) DeleteIngress(namespace string, name string) error {
	client, err := c.KubernetesClientSet()
	if err != nil {
		return err
	}
	return client.ExtensionsV1beta1().Ingresses(namespace).Delete(context.TODO(), name, meta_v1.DeleteOptions{})
}

func (c *client) GetLogs(namespace string, pod string, containerName string, follow bool, previous bool, line int64) (io.ReadCloser, error) {
	var tailLine *int64
	if line == -1 {
		tailLine = nil
	} else {
		tailLine = &line
	}
	req := c.client.CoreV1().Pods(namespace).GetLogs(
		pod,
		&core_v1.PodLogOptions{
			Follow:    follow,
			Container: containerName,
			TailLines: tailLine,
			Previous:  previous,
		},
	)
	readCloser, err := req.Stream(context.TODO())
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
	pod, err := c.client.CoreV1().Pods(namespace).Get(context.TODO(), podName, meta_v1.GetOptions{})
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

func (c *client) GetSelectRelationPod(info *resource.Info) (map[string][]core_v1.Pod, error) {
	objPods := make(map[string][]core_v1.Pod)
	if info == nil {
		return nil, nil
	}

	selector, ok := getSelectorFromObject(info.Object)
	if !ok {
		return nil, nil
	}

	pods, err := c.client.CoreV1().Pods(info.Namespace).List(context.TODO(), meta_v1.ListOptions{
		FieldSelector: fields.Everything().String(),
		LabelSelector: labels.Set(selector).AsSelector().String(),
	})
	if err != nil {
		return nil, err
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
		obj, err := labelAndAnnotationsRepoObj(info, namespace, version, c, commit)

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

		newManifestBuf.WriteString("\n---\n")
		newManifestBuf.Write(objB)
	}

	return newManifestBuf, nil
}

func (c *client) ConvertCertificate(namespace, manifest, commit string) (*bytes.Buffer, error, bool) {
	converted := false
	result, err := c.BuildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err), converted
	}

	newManifestBuf := bytes.NewBuffer(nil)
	for _, info := range result {

		obj := info.Object.(*unstructured.Unstructured)

		certExist := existCert(obj.Object)
		if !certExist {
			objB, err := yaml.Marshal(obj)
			if err != nil {
				return nil, fmt.Errorf("yaml marshal: %v", err), converted
			}
			newManifestBuf.WriteString("\n---\n")
			newManifestBuf.Write(objB)
			continue
		}

		certInfo := getCertInfo(obj.Object)
		key := certInfo["key"].(string)
		cert := certInfo["cert"].(string)

		secretLabels := obj.GetLabels()
		if secretLabels == nil {
			secretLabels = make(map[string]string)
		}

		secretLabels[model.TlsSecretLabel] = model.AgentVersion

		secretAnnotations := obj.GetAnnotations()
		if secretAnnotations == nil {
			secretAnnotations = make(map[string]string)
		}

		objectMeta := meta_v1.ObjectMeta{
			Name:        obj.GetName(),
			Namespace:   namespace,
			Labels:      secretLabels,
			Annotations: secretAnnotations,
		}

		typeMeta := meta_v1.TypeMeta{
			Kind:       "Secret",
			APIVersion: "v1",
		}

		data := make(map[string][]byte)

		data["tls.key"] = []byte(key)
		data["tls.crt"] = []byte(cert)

		tlsSecret := core_v1.Secret{
			TypeMeta:   typeMeta,
			ObjectMeta: objectMeta,
			Data:       data,
			Type:       "kubernetes.io/tls",
		}

		objB, err := yaml.Marshal(tlsSecret)
		if err != nil {
			return nil, fmt.Errorf("yaml marshal: %v", err), converted
		}

		newManifestBuf.WriteString("\n---\n")
		newManifestBuf.Write(objB)
		converted = true
	}

	return newManifestBuf, nil, converted
}

// 给资源加标签和注解
func labelAndAnnotationsRepoObj(info *resource.Info, namespace, version string, c *client, commit string) (runtime.Object, error) {

	obj := info.Object.(*unstructured.Unstructured)

	l := obj.GetLabels()
	annotations := obj.GetAnnotations()

	if l == nil {
		l = make(map[string]string)
	}

	if annotations == nil {
		annotations = make(map[string]string)
	}

	defer obj.SetLabels(l)
	defer obj.SetAnnotations(annotations)

	var addTemplateAppLabels = func(workloadKind, workloadName string) {
		tplLabels := getTemplateLabels(obj.Object)
		tplLabels[model.ParentWorkloadLabel] = workloadKind
		tplLabels[model.ParentWorkloadNameLabel] = workloadName
		tplLabels[model.CommitLabel] = commit
		if err := setTemplateLabels(obj.Object, tplLabels); err != nil {
			glog.Warningf("Set Template Labels failed, %v", err)
		}
	}

	var addCronJobTemplateAppLabels = func(workloadKind, workloadName string) {
		tplLabels := getCronJobPodTemplateLabels(obj.Object)
		tplLabels[model.ParentWorkloadLabel] = workloadKind
		tplLabels[model.ParentWorkloadNameLabel] = workloadName
		tplLabels[model.CommitLabel] = commit
		if err := setCronJobPodTemplateLabels(obj.Object, tplLabels); err != nil {
			glog.Warningf("Set Template Labels failed, %v", err)
		}
	}

	annotations[model.CommitLabel] = commit

	kind := info.Mapping.GroupVersionKind.Kind

	switch kind {
	case "Service":
		l[model.NetworkLabel] = "service"
	case "Ingress":
		l[model.NetworkLabel] = "ingress"
	case "ConfigMap", "Secret", "Certificate":
	case "C7NHelmRelease":
		if namespace == model.AgentNamespace {
			l[model.C7NHelmReleaseClusterLabel] = model.ClusterId
		}
		// 从集群中查出C7NHelmRelease，如果资源不存在，添加Annotation["choerodon.io/C7NHelmRelease-status"]="INSTALL"，即安装操作
		// 如果资源存在，判断已更新的spec和集群存在的spec是否相同,相同不做任何操作,即不会进行升级操作
		// 不同添加Annotation["choerodon.io/C7NHelmRelease-status"]="UPGRADE",即进行升级操作
		c7nHelmRelease := c.GetC7NHelmRelease(obj.GetName(), namespace)
		newSpecMap := getSpecField(obj.Object)

		if c7nHelmRelease == nil {
			annotations[model.C7NHelmReleaseOperateAnnotation] = model.INSTALL
		} else {
			oldSpec := v1alpha1.C7NHelmReleaseSpec{}
			newSpec := v1alpha1.C7NHelmReleaseSpec{}

			// 获得集群中C7nHelmRelease的spec
			oldSpec = c7nHelmRelease.Spec

			// 获得更新后的spec
			newSpecJsonByte, err := json.Marshal(newSpecMap)
			if err != nil {
				glog.Info(err)
				return nil, err
			}
			if err := json.Unmarshal(newSpecJsonByte, &newSpec); err != nil {
				glog.Info(err)
				return nil, err
			}

			glog.Infof("oldSpecChartName:%s newSpecChartName:%s", oldSpec.ChartName, newSpec.ChartName)

			// 表示市场应用的跨服务升级
			if oldSpec.ChartName != newSpec.ChartName {
				annotations[model.C7NHelmReleaseOperateAnnotation] = model.CROSS_UPGRADE
				break
			}

			if !reflect.DeepEqual(newSpec, oldSpec) {
				annotations[model.C7NHelmReleaseOperateAnnotation] = model.UPGRADE
			}
		}
	case "PersistentVolumeClaim":
		l[model.PvcLabel] = fmt.Sprintf(model.PvcLabelValueFormat, model.ClusterId)
		l[model.NameLabel] = obj.GetName()
	case "PersistentVolume":
		l[model.EnvLabel] = namespace
		l[model.PvLabel] = fmt.Sprintf(model.PvLabelValueFormat, model.ClusterId)
		l[model.NameLabel] = obj.GetName()
	case "Deployment", "StatefulSet", "DaemonSet", "Job":
		l[model.WorkloadLabel] = kind
		addTemplateAppLabels(kind, obj.GetName())
	case "CronJob":
		l[model.WorkloadLabel] = kind
		addCronJobTemplateAppLabels(kind, obj.GetName())
	default:
		glog.Warningf("not support add label for object : %v", obj)
		return obj, nil
	}
	l[model.AgentVersionLabel] = model.AgentVersion

	return obj, nil
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

func getCronJobPodTemplateLabels(obj map[string]interface{}) map[string]string {
	tplLabels, _, err := unstructured.NestedStringMap(obj, "spec", "jobTemplate", "spec", "template", "metadata", "labels")
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

func setCronJobPodTemplateLabels(obj map[string]interface{}, templateLabels map[string]string) error {
	return unstructured.SetNestedStringMap(obj, templateLabels, "spec", "jobTemplate", "spec", "template", "metadata", "labels")
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

func existCert(obj map[string]interface{}) bool {
	_, exist, err := unstructured.NestedMap(obj, "spec", "existCert")
	if err != nil {
		glog.Warningf("Get existCert field failed, %v", err)
	}
	return exist
}

func getCertInfo(obj map[string]interface{}) map[string]interface{} {
	cert, _, err := unstructured.NestedMap(obj, "spec", "existCert")
	if err != nil {
		glog.Warningf("Get existCert field failed, %v", err)
	}
	return cert
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
	switch kind {
	case "ReplicationController":
		matchLabels, result, err := unstructured.NestedStringMap(t.Object, "spec", "selector")
		if result == false && err != nil {
			return nil, false
		}
		return matchLabels, true
	case "Deployment", "Job", "ReplicaSet", "DaemonSet", "StatefulSet":
		matchLabels, result, err := unstructured.NestedStringMap(t.Object, "spec", "selector", "matchLabels")
		if result == false && err != nil {
			return nil, false
		}
		return matchLabels, true
	default:
		return nil, false
	}
}

func (c *client) CreateOrUpdateDockerRegistrySecret(namespace string, secret *core_v1.Secret) (*core_v1.Secret, error) {
	_, err := c.client.CoreV1().Secrets(namespace).Get(context.TODO(), secret.Name, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return c.client.CoreV1().Secrets(namespace).Create(context.TODO(), secret, meta_v1.CreateOptions{})
		}
		return nil, err
	}
	return c.client.CoreV1().Secrets(namespace).Update(context.TODO(), secret, meta_v1.UpdateOptions{})
}

func InArray(expectedResourceKind []string, kind string) bool {
	for _, item := range expectedResourceKind {
		if item == kind {
			return true
		}
	}
	return false
}
func (c *client) GetDeployment(namespace string, deploymentName string) (string, error) {
	deployment, err := c.client.AppsV1().Deployments(namespace).Get(context.TODO(), deploymentName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if deployment.Annotations != nil && deployment.Annotations[model.CommitLabel] != "" {
		return deployment.Annotations[model.CommitLabel], nil
	}
	return "", nil
}
func (c *client) GetStatefulSet(namespace string, statefulSetName string) (string, error) {
	statefulSet, err := c.client.AppsV1().StatefulSets(namespace).Get(context.TODO(), statefulSetName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if statefulSet.Annotations != nil && statefulSet.Annotations[model.CommitLabel] != "" {
		return statefulSet.Annotations[model.CommitLabel], nil
	}
	return "", nil
}
func (c *client) GetJob(namespace string, jobName string) (string, error) {
	job, err := c.client.BatchV1().Jobs(namespace).Get(context.TODO(), jobName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if job.Annotations != nil && job.Annotations[model.CommitLabel] != "" {
		return job.Annotations[model.CommitLabel], nil
	}
	return "", nil
}
func (c *client) GetCronJob(namespace string, cronJobName string) (string, error) {
	cronJob, err := c.client.BatchV1beta1().CronJobs(namespace).Get(context.TODO(), cronJobName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if cronJob.Annotations != nil && cronJob.Annotations[model.CommitLabel] != "" {
		return cronJob.Annotations[model.CommitLabel], nil
	}
	return "", nil
}
func (c *client) GetDaemonSet(namespace string, daemonSetName string) (string, error) {
	daemon, err := c.client.AppsV1().DaemonSets(namespace).Get(context.TODO(), daemonSetName, meta_v1.GetOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return "", err
		}
		return "", nil
	}
	if daemon.Annotations != nil && daemon.Annotations[model.CommitLabel] != "" {
		return daemon.Annotations[model.CommitLabel], nil
	}
	return "", nil
}
