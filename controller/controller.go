package controller

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	agentnamespace "github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/metrics"
	metricsnode "github.com/choerodon/choerodon-cluster-agent/pkg/metrics/node"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/tools/cache"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/choerodon/choerodon-cluster-agent/controller/endpoint"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
)

type InitFunc func(ctx *ControllerContext) (bool, error)

var controllers = map[string]InitFunc{}

const workers int = 1

type ControllerContext struct {
	kubeInformer   kubeinformers.SharedInformerFactory
	kubeClientset  clientset.Interface
	kubeClient     kube.Client
	helmClient     helm.Client
	stop           chan struct{}
	stopController <-chan struct{}
	chans          *channel.CRChan
	Namespaces     *agentnamespace.Namespaces
	informers      []cache.SharedIndexInformer
	PlatformCode   string
}

func init() {
	controllers["endpoint"] = startEndpointController
	controllers["deployment"] = startDeploymentController
	controllers["job"] = startJobController
	controllers["service"] = startServiceController
	controllers["secret"] = startSecretController
	controllers["configmap"] = startConfigMapController
	controllers["ingress"] = startIngressController
	controllers["replicaset"] = startReplicaSetController
	controllers["pod"] = startPodController
	controllers["c7nhelmrelease"] = startC7NHelmReleaseController
	controllers["namesapce"] = startNamespaceController
	controllers["daemonset"] = startDaemonSetController
	controllers["statefulset"] = startStatefulSetController
	controllers["node"] = startNodeController
}

func CreateControllerContext(
	kubeClientset clientset.Interface,
	kubeClient kube.Client,
	helmClient helm.Client,
	stop <-chan struct{},
	chans *channel.CRChan,
	Namespaces *agentnamespace.Namespaces,
	platformCode string) *ControllerContext {

	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClientset, time.Second*30)

	ctx := &ControllerContext{
		kubeInformer:   kubeInformer,
		kubeClientset:  kubeClientset,
		kubeClient:     kubeClient,
		helmClient:     helmClient,
		stopController: stop,
		stop:           make(chan struct{}, 1),
		Namespaces:     Namespaces,
		chans:          chans,
		informers:      []cache.SharedIndexInformer{},
		PlatformCode:   platformCode,
	}
	return ctx
}

func (ctx *ControllerContext) StartControllers() error {

	glog.Infof("Starting controllers for envs %v", ctx.Namespaces.GetAll())
	// for choerodon test-manager auto test
	ctx.Namespaces.Add("choerodon-test")
	go func() {
		for {
			for controllerName, initFn := range controllers {
				started, err := initFn(ctx)
				if err != nil {
					glog.Errorf("Error starting %q :%v", controllerName, err)
				}
				if !started {
					glog.Warningf("Skipping %q", controllerName)
					continue
				}
			}
			ctx.kubeInformer.Start(ctx.stop)
			select {
			case <-ctx.stopController:
				glog.Infof("Stopping controllers")
				close(ctx.stop)
				return
			case <-ctx.stop:
				return

			}
		}
	}()
	go metrics.Run(ctx.stop)
	return nil
}

func (ctx *ControllerContext) ReSync() {
	close(ctx.stop)
	ctx.stop = make(chan struct{}, 1)
	ctx.kubeInformer = kubeinformers.NewSharedInformerFactory(ctx.kubeClientset, time.Second*30)
	ctx.StartControllers()

}

func startEndpointController(ctx *ControllerContext) (bool, error) {
	go endpoint.NewEndpointController(
		ctx.kubeInformer.Core().V1().Pods(),
		ctx.kubeInformer.Core().V1().Services(),
		ctx.kubeInformer.Core().V1().Endpoints(),
		ctx.kubeClientset,
	).Run(workers, ctx.stop)
	return true, nil
}
func startNamespaceController(ctx *ControllerContext) (bool, error) {

	fmt.Println("start namespace has deleted ")
	return true, nil
}

func startDeploymentController(ctx *ControllerContext) (bool, error) {

	fmt.Println("start deployment has moved ")
	return true, nil
}

func startDaemonSetController(ctx *ControllerContext) (bool, error) {
	fmt.Println("start daemonset has moved ")
	return true, nil
}

func startStatefulSetController(ctx *ControllerContext) (bool, error) {

	go func() {
		namespaces := ctx.Namespaces.GetAll()
		for _, ns := range namespaces {
			pods, err := ctx.kubeInformer.Apps().V1().StatefulSets().Lister().StatefulSets(ns).List(labels.NewSelector())
			if err != nil {
				glog.Fatal("can not list resource, no rabc bind, exit !")
			} else {
				var podList []string
				for _, pod := range pods {
					if pod.Labels[model.ReleaseLabel] != "" {
						podList = append(podList, pod.GetName())
					}
				}
				resourceList := &kubernetes.ResourceList{
					Resources:    podList,
					ResourceType: "StatefulSet",
				}
				content, err := json.Marshal(resourceList)
				if err != nil {
					glog.Fatal("marshal pod list error")
				} else {
					response := &model.Packet{
						Key:     fmt.Sprintf("env:%s", ns),
						Type:    model.ResourceSync,
						Payload: string(content),
					}
					ctx.chans.ResponseChan <- response
				}
			}
		}
	}()
	return false, fmt.Errorf("no  group version for statefulset compatiable")
}

func startIngressController(ctx *ControllerContext) (bool, error) {
	fmt.Println("start ingress has moved ")
	return true, nil
}

func startReplicaSetController(ctx *ControllerContext) (bool, error) {

	go func() {
		namespaces := ctx.Namespaces.GetAll()
		for _, ns := range namespaces {
			lister := ctx.kubeInformer.Extensions().V1beta1().ReplicaSets().Lister()
			rsList, err := lister.ReplicaSets(ns).List(labels.NewSelector())
			if err != nil {
				glog.Fatal("can not list resource, no rabc bind, exit !")
			} else {
				var resourceSyncList []string
				for _, resource := range rsList {
					if resource.Labels[model.ReleaseLabel] != "" {
						resourceSyncList = append(resourceSyncList, resource.GetName())
					}
				}
				resourceList := &kubernetes.ResourceList{
					Resources:    resourceSyncList,
					ResourceType: "ReplicaSet",
				}
				content, err := json.Marshal(resourceList)
				if err != nil {
					glog.Fatal("marshal ReplicaSet list error")
				} else {
					response := &model.Packet{
						Key:     fmt.Sprintf("env:%s", ns),
						Type:    model.ResourceSync,
						Payload: string(content),
					}
					ctx.chans.ResponseChan <- response
				}
			}
		}

	}()
	return true, nil
}

func startJobController(ctx *ControllerContext) (bool, error) {
	fmt.Println("start job has moved")
	return true, nil
}

func startServiceController(ctx *ControllerContext) (bool, error) {

	namespaces := ctx.Namespaces.GetAll()

	for _, ns := range namespaces {
		instances, err := ctx.kubeClientset.CoreV1().Services(ns).List(v1.ListOptions{})
		if err != nil {
			glog.Fatal(err)
		} else {
			var serviceList []string
			for _, instance := range instances.Items {
				if instance.Labels[model.ReleaseLabel] != "" {
					serviceList = append(serviceList, instance.GetName())
				}
			}
			resourceList := &kubernetes.ResourceList{
				Resources:    serviceList,
				ResourceType: "Service",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal service list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.chans.ResponseChan <- response
			}
		}
	}
	return true, nil
}

func startSecretController(ctx *ControllerContext) (bool, error) {
	fmt.Println("start secret has moved")
	return true, nil
}

func startConfigMapController(ctx *ControllerContext) (bool, error) {
	fmt.Println("start configmap has moved")
	return true, nil
}

func startPodController(ctx *ControllerContext) (bool, error) {
	go func() {
		namespaces := ctx.Namespaces.GetAll()
		for _, ns := range namespaces {
			pods, err := ctx.kubeInformer.Core().V1().Pods().Lister().Pods(ns).List(labels.NewSelector())
			if err != nil {
				glog.Fatal("can not list resource, no rabc bind, exit !")
			} else {
				var podList []string
				for _, pod := range pods {
					if pod.Labels[model.ReleaseLabel] != "" {
						podList = append(podList, pod.GetName())
					}
				}
				resourceList := &kubernetes.ResourceList{
					Resources:    podList,
					ResourceType: "Pod",
				}
				content, err := json.Marshal(resourceList)
				if err != nil {
					glog.Fatal("marshal pod list error")
				} else {
					response := &model.Packet{
						Key:     fmt.Sprintf("env:%s", ns),
						Type:    model.ResourceSync,
						Payload: string(content),
					}
					ctx.chans.ResponseChan <- response
				}
			}
		}
	}()
	return true, nil
}

// todo move to another place
func startNodeController(ctx *ControllerContext) (bool, error) {
	m := &metricsnode.Node{
		Client: ctx.kubeClient.GetKubeClient(),
		CrChan: ctx.chans,
	}

	metrics.Register(m)

	return true, nil
}

func startC7NHelmReleaseController(ctx *ControllerContext) (bool, error) {
	go fmt.Println("start c7n helm release has moved")
	return true, nil
}
