package cmd

import (
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/choerodon/choerodon-agent/pkg/controller/configmap"
	"github.com/choerodon/choerodon-agent/pkg/controller/deployment"
	"github.com/choerodon/choerodon-agent/pkg/controller/endpoint"
	"github.com/choerodon/choerodon-agent/pkg/controller/ingress"
	"github.com/choerodon/choerodon-agent/pkg/controller/job"
	"github.com/choerodon/choerodon-agent/pkg/controller/pod"
	"github.com/choerodon/choerodon-agent/pkg/controller/replicaset"
	"github.com/choerodon/choerodon-agent/pkg/controller/secret"
	"github.com/choerodon/choerodon-agent/pkg/controller/service"

	chrclientset "github.com/choerodon/choerodon-agent/pkg/client/clientset/versioned"
	c7ninformers "github.com/choerodon/choerodon-agent/pkg/client/informers/externalversions"
	"github.com/choerodon/choerodon-agent/pkg/controller/c7nhelmrelease"
	"github.com/choerodon/choerodon-agent/pkg/controller/event"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
)

type InitFunc func(ctx *ControllerContext) (bool, error)

type ControllerContext struct {
	options       *AgentRunOptions
	kubeInformer  kubeinformers.SharedInformerFactory
	kubeClientset clientset.Interface
	c7nClientset  chrclientset.Interface
	c7nInformer   c7ninformers.SharedInformerFactory
	kubeClient    kube.Client
	helmClient    helm.Client
	stop          <-chan struct{}
	commandChan   chan<- *model.Command
	responseChan  chan<- *model.Response
	namespace     string
}

func CreateControllerContext(
	options *AgentRunOptions,
	kubeClientset clientset.Interface,
	c7nClientset chrclientset.Interface,
	kubeClient kube.Client,
	helmClient helm.Client,
	stop <-chan struct{},
	commandChan chan<- *model.Command,
	responseChan chan<- *model.Response) *ControllerContext {
	kubeInformer := kubeinformers.NewFilteredSharedInformerFactory(kubeClientset, time.Second*30, options.Namespace, nil)
	c7nInformer := c7ninformers.NewFilteredSharedInformerFactory(c7nClientset, time.Second*30, options.Namespace, nil)

	ctx := &ControllerContext{
		options:       options,
		kubeInformer:  kubeInformer,
		kubeClientset: kubeClientset,
		c7nClientset:  c7nClientset,
		c7nInformer:   c7nInformer,
		kubeClient:    kubeClient,
		helmClient:    helmClient,
		stop:          stop,
		commandChan:   commandChan,
		responseChan:  responseChan,
		namespace:     options.Namespace,
	}
	return ctx
}

func NewControllerInitializers() map[string]InitFunc {
	controllers := map[string]InitFunc{}
	controllers["endpoint"] = startEndpointController
	controllers["deployment"] = startDeploymentController
	controllers["job"] = startJobController
	controllers["service"] = startServiceController
	controllers["secret"] = startSecretController
	controllers["configmap"] = startConfigMapController
	controllers["ingress"] = startIngressController
	controllers["replicaset"] = startReplicaSetController
	controllers["pod"] = startPodController
	controllers["event"] = startEventController
	controllers["c7nhelmrelease"] = startC7NHelmReleaseController
	return controllers
}

func StartControllers(ctx *ControllerContext, controllers map[string]InitFunc) error {
	for controllerName, initFn := range controllers {
		glog.V(1).Infof("Starting %q", controllerName)
		started, err := initFn(ctx)
		if err != nil {
			glog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
		glog.Infof("Started %q", controllerName)
	}
	return nil
}

func startEndpointController(ctx *ControllerContext) (bool, error) {
	go endpoint.NewEndpointController(
		ctx.kubeInformer.Core().V1().Pods(),
		ctx.kubeInformer.Core().V1().Services(),
		ctx.kubeInformer.Core().V1().Endpoints(),
		ctx.kubeClientset,
	).Run(int(ctx.options.ConcurrentEndpointSyncs), ctx.stop)
	return true, nil
}

func startDeploymentController(ctx *ControllerContext) (bool, error) {
	go deployment.NewDeploymentController(
		ctx.kubeInformer.Extensions().V1beta1().Deployments(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentDeploymentSyncs), ctx.stop)
	return true, nil
}

func startIngressController(ctx *ControllerContext) (bool, error) {
	go ingress.NewIngressController(
		ctx.kubeInformer.Extensions().V1beta1().Ingresses(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentIngressSyncs), ctx.stop)
	return true, nil
}

func startReplicaSetController(ctx *ControllerContext) (bool, error) {
	go replicaset.NewReplicaSetController(
		ctx.kubeInformer.Extensions().V1beta1().ReplicaSets(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentRSSyncs), ctx.stop)
	return true, nil
}

func startJobController(ctx *ControllerContext) (bool, error) {
	go job.NewJobController(
		ctx.kubeInformer.Batch().V1().Jobs(),
		ctx.kubeClient,
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentJobSyncs), ctx.stop)
	return true, nil
}

func startServiceController(ctx *ControllerContext) (bool, error) {
	go service.NewserviceController(
		ctx.kubeInformer.Core().V1().Services(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentServiceSyncs), ctx.stop)
	return true, nil
}

func startSecretController(ctx *ControllerContext) (bool, error) {
	go secret.NewSecretController(
		ctx.kubeInformer.Core().V1().Secrets(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentSecretSyncs), ctx.stop)
	return true, nil
}

func startConfigMapController(ctx *ControllerContext) (bool, error) {
	go configMap.NewconfigMapController(
		ctx.kubeInformer.Core().V1().ConfigMaps(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentConfigMapSyncs), ctx.stop)
	return true, nil
}

func startPodController(ctx *ControllerContext) (bool, error) {
	go pod.NewpodController(
		ctx.kubeInformer.Core().V1().Pods(),
		ctx.responseChan,
		ctx.namespace,
	).Run(int(ctx.options.ConcurrentPodSyncs), ctx.stop)
	return true, nil
}

func startC7NHelmReleaseController(ctx *ControllerContext) (bool, error) {
	go c7nhelmrelease.NewController(
		ctx.kubeClientset,
		ctx.c7nClientset,
		ctx.c7nInformer.Choerodon().V1alpha1().C7NHelmReleases(),
		ctx.helmClient,
		ctx.commandChan,
	).Run(int(ctx.options.ConcurrentC7NHelmReleaseSyncs), ctx.stop)
	return true, nil
}

func startEventController(ctx *ControllerContext) (bool, error) {
	go event.NewEventController(
		ctx.kubeInformer.Core().V1().Events(),
		ctx.responseChan,
		ctx.namespace,
		ctx.kubeClientset,
	).Run(int(ctx.options.ConcurrentPodSyncs), ctx.stop)
	return true, nil
}
