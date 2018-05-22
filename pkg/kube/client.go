package kube

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"

	"github.com/golang/glog"
	core_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/kubernetes/pkg/api/legacyscheme"
	"k8s.io/kubernetes/pkg/client/clientset_generated/internalclientset"
	"k8s.io/kubernetes/pkg/kubectl"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"k8s.io/kubernetes/pkg/kubectl/resource"

	model_helm "github.com/choerodon/choerodon-agent/pkg/model/helm"
)

type Client interface {
	DeleteJob(namespace string, name string) error
	LogsForJob(namespace string, name string) (string, error)
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
}

type client struct {
	cmdutil.Factory
	client *kubernetes.Clientset
}

func NewClient(f cmdutil.Factory) (Client, error) {
	kubeClient, err := f.KubernetesClientSet()
	if err != nil {
		return nil, fmt.Errorf("get kubernetes client: %v", err)
	}

	return &client{
		Factory: f,
		client:  kubeClient,
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

func (c *client) GetResources(namespace string, manifest string) ([]*model_helm.ReleaseResource, error) {
	resources := make([]*model_helm.ReleaseResource, 0, 10)
	result, err := c.buildUnstructured(namespace, manifest)
	if err != nil {
		return nil, fmt.Errorf("build unstructured: %v", err)
	}
	for _, info := range result {
		if err := info.Get(); err != nil {
			glog.Errorf("WARNING: Failed Get for resource %q: %s", info.Name, err)
			continue
		}
		hrr := &model_helm.ReleaseResource{
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
			glog.Error(err)
		}
		resources = append(resources, hrr)
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

func (c *client) LogsForJob(namespace string, name string) (string, error) {
	job, err := c.client.BatchV1().Jobs(namespace).Get(name, meta_v1.GetOptions{})
	if err != nil {
		return "", err
	}
	selector, err := meta_v1.LabelSelectorAsSelector(job.Spec.Selector)
	if err != nil {
		return "", err
	}
	podList, err := c.client.CoreV1().Pods(namespace).List(meta_v1.ListOptions{LabelSelector: selector.String()})
	if err != nil {
		return "", err
	}
	if len(podList.Items) == 0 {
		return "", nil
	}
	pod := podList.Items[0]
	req := c.client.CoreV1().Pods(namespace).GetLogs(pod.Name, &core_v1.PodLogOptions{})
	readCloser, err := req.Stream()
	if err != nil {
		return "", err
	}
	defer readCloser.Close()
	buf := new(bytes.Buffer)
	io.Copy(buf, readCloser)
	return buf.String(), nil
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
	req := c.client.CoreV1().Pods(namespace).GetLogs(
		pod,
		&core_v1.PodLogOptions{
			Follow:     true,
			Timestamps: true,
			Container:  containerName,
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
		return err
	}
	if pod.Status.Phase == core_v1.PodSucceeded || pod.Status.Phase == core_v1.PodFailed {
		return fmt.Errorf("cannot exec into a container in a completed pod; current phase is %s", pod.Status.Phase)
	}
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
		Command:   []string{"/bin/sh", "-c", "TERM=xterm exec $( (type getent > /dev/null 2>&1  && getent passwd root | cut -d: -f7 2>/dev/null) || echo /bin/sh)"},
	}, legacyscheme.ParameterCodec)
	return execute(http.MethodPost, req.URL(), config, local, local, local, true)
}
