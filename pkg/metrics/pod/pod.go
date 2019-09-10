package pod

import (
	"encoding/json"
	"errors"
	"fmt"
	"k8s.io/kubernetes/pkg/kubectl/metricsutil"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/kubernetes/pkg/kubectl/cmd/top"
	metricsapi "k8s.io/metrics/pkg/apis/metrics"
	metricsv1beta1api "k8s.io/metrics/pkg/apis/metrics/v1beta1"
	metricsclientset "k8s.io/metrics/pkg/client/clientset/versioned"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	agentnamespace "github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
)

const metricsCreationDelay = 2 * time.Minute

type Pod struct {
	Client     kube.Client
	CrChan     *channel.CRChan
	Namespaces *agentnamespace.Namespaces
}

func (po *Pod) Run(stopCh <-chan struct{}) error {
	responseChan := po.CrChan.ResponseChan
	discoveryInterface, e := po.Client.GetDiscoveryClient()
	if e != nil {
		glog.Errorf("Get Discovery Client error: %s", e.Error())
		return e
	}
	config, e := po.Client.GetRESTConfig()
	if e != nil {
		glog.Errorf("Get REST Config error: %s", e.Error())
		return e
	}
	metricsClient, e := metricsclientset.NewForConfig(config)
	if e != nil {
		glog.Errorf("Get Metrics Client error: %s", e.Error())
		return e
	}
	o := top.TopPodOptions{
		DiscoveryClient: discoveryInterface,
		MetricsClient:   metricsClient,
		ResourceName:    "",
		Client: metricsutil.NewHeapsterMetricsClient(
			po.Client.GetKubeClient().CoreV1(),
			metricsutil.DefaultHeapsterNamespace,
			metricsutil.DefaultHeapsterScheme,
			metricsutil.DefaultHeapsterService,
			""),
		AllNamespaces:   false,
		PrintContainers: false,
		NoHeaders:       false,
	}
	o.PodClient = po.Client.GetKubeClient().CoreV1()
	for {
		select {
		case <-stopCh:
			glog.Info("stop pod metrics collector")
			return nil
		case <-time.Tick(time.Second * 30):
			for _, namespace := range po.Namespaces.GetAll() {
				o.Namespace = namespace
				list, e := runTopPod(o)
				if e != nil {
					glog.Errorf("Get REST Config error: %s", e.Error())
					return e
				}
				content, e := json.Marshal(list)
				if e != nil {
					glog.Error("marshal pod metrics list error")
				} else {
					response := &model.Packet{
						Key:     fmt.Sprintf("env:%s", namespace),
						Type:    model.PodMetricsSync,
						Payload: string(content),
					}
					responseChan <- response
				}
			}
		}
	}
}

func runTopPod(o top.TopPodOptions) (*metricsapi.PodMetricsList, error) {
	var err error
	selector := labels.Everything()
	if len(o.Selector) > 0 {
		selector, err = labels.Parse(o.Selector)
		if err != nil {
			return nil, err
		}
	}

	apiGroups, err := o.DiscoveryClient.ServerGroups()
	if err != nil {
		return nil, err
	}

	metricsAPIAvailable := top.SupportedMetricsAPIVersionAvailable(apiGroups)

	metrics := &metricsapi.PodMetricsList{}
	if metricsAPIAvailable {
		metrics, err = getMetricsFromMetricsAPI(o.MetricsClient, o.Namespace, o.ResourceName, o.AllNamespaces, selector)
		if err != nil {
			return nil, err
		}
	} else {
		metrics, err = o.Client.GetPodMetrics(o.Namespace, o.ResourceName, o.AllNamespaces, selector)
		if err != nil {
			return nil, err
		}
	}

	// TODO: Refactor this once Heapster becomes the API server.
	// First we check why no metrics have been received.
	if len(metrics.Items) == 0 {
		// If the API server query is successful but all the pods are newly created,
		// the metrics are probably not ready yet, so we return the error here in the first place.
		e := verifyEmptyMetrics(o, selector)
		if e != nil {
			return nil, e
		}
	}
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func getMetricsFromMetricsAPI(metricsClient metricsclientset.Interface, namespace, resourceName string, allNamespaces bool, selector labels.Selector) (*metricsapi.PodMetricsList, error) {
	var err error
	ns := metav1.NamespaceAll
	if !allNamespaces {
		ns = namespace
	}
	versionedMetrics := &metricsv1beta1api.PodMetricsList{}
	if resourceName != "" {
		m, err := metricsClient.MetricsV1beta1().PodMetricses(ns).Get(resourceName, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		versionedMetrics.Items = []metricsv1beta1api.PodMetrics{*m}
	} else {
		versionedMetrics, err = metricsClient.MetricsV1beta1().PodMetricses(ns).List(metav1.ListOptions{LabelSelector: selector.String()})
		if err != nil {
			return nil, err
		}
	}
	metrics := &metricsapi.PodMetricsList{}
	err = metricsv1beta1api.Convert_v1beta1_PodMetricsList_To_metrics_PodMetricsList(versionedMetrics, metrics, nil)
	if err != nil {
		return nil, err
	}
	return metrics, nil
}

func verifyEmptyMetrics(o top.TopPodOptions, selector labels.Selector) error {
	pods, err := o.PodClient.Pods(o.Namespace).List(metav1.ListOptions{
		LabelSelector: selector.String(),
	})
	if err != nil {
		return err
	}
	if len(pods.Items) == 0 {
		return nil
	}
	for _, pod := range pods.Items {
		if err := checkPodAge(&pod); err != nil {
			return err
		}
	}
	return errors.New("metrics not available yet")
}

func checkPodAge(pod *v1.Pod) error {
	age := time.Since(pod.CreationTimestamp.Time)
	if age > metricsCreationDelay {
		message := fmt.Sprintf("Metrics not available for pod %s/%s, age: %s", pod.Namespace, pod.Name, age.String())
		glog.Warningf(message)
		return errors.New(message)
	} else {
		glog.V(2).Infof("Metrics not yet available for pod %s/%s, age: %s", pod.Namespace, pod.Name, age.String())
		return nil
	}
}
