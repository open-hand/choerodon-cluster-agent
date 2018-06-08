package cmd

import (
	"time"

	"github.com/golang/glog"
	"k8s.io/client-go/informers"
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
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
)

type InitFunc func(ctx *ControllerContext) (bool, error)

type ControllerContext struct {
	Options         *AgentRunOptions
	InformerFactory informers.SharedInformerFactory
	Client          clientset.Interface
	KubeClient      kube.Client
	Stop            <-chan struct{}
	ResponseChan    chan<- *model.Response
	Namespace       string
}

func CreateControllerContext(
	options *AgentRunOptions,
	client clientset.Interface,
	kubeClient kube.Client,
	stop <-chan struct{},
	responseChan chan<- *model.Response) *ControllerContext {
	informerFactory := informers.NewFilteredSharedInformerFactory(client, time.Second*30, options.Namespace, nil)

	ctx := &ControllerContext{
		Options:         options,
		InformerFactory: informerFactory,
		Client:          client,
		Stop:            stop,
		KubeClient:      kubeClient,
		ResponseChan:    responseChan,
		Namespace:       options.Namespace,
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
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.InformerFactory.Core().V1().Services(),
		ctx.InformerFactory.Core().V1().Endpoints(),
		ctx.Client,
	).Run(int(ctx.Options.ConcurrentEndpointSyncs), ctx.Stop)
	return true, nil
}

func startDeploymentController(ctx *ControllerContext) (bool, error) {
	go deployment.NewDeploymentController(
		ctx.InformerFactory.Extensions().V1beta1().Deployments(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentDeploymentSyncs), ctx.Stop)
	return true, nil
}

func startIngressController(ctx *ControllerContext) (bool, error) {
	go ingress.NewIngressController(
		ctx.InformerFactory.Extensions().V1beta1().Ingresses(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentIngressSyncs), ctx.Stop)
	return true, nil
}

func startReplicaSetController(ctx *ControllerContext) (bool, error) {
	go replicaset.NewReplicaSetController(
		ctx.InformerFactory.Extensions().V1beta1().ReplicaSets(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentRSSyncs), ctx.Stop)
	return true, nil
}

func startJobController(ctx *ControllerContext) (bool, error) {
	go job.NewJobController(
		ctx.InformerFactory.Batch().V1().Jobs(),
		ctx.KubeClient,
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentJobSyncs), ctx.Stop)
	return true, nil
}

func startServiceController(ctx *ControllerContext) (bool, error) {
	go service.NewserviceController(
		ctx.InformerFactory.Core().V1().Services(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentServiceSyncs), ctx.Stop)
	return true, nil
}

func startSecretController(ctx *ControllerContext) (bool, error) {
	go secret.NewSecretController(
		ctx.InformerFactory.Core().V1().Secrets(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentSecretSyncs), ctx.Stop)
	return true, nil
}

func startConfigMapController(ctx *ControllerContext) (bool, error) {
	go configMap.NewconfigMapController(
		ctx.InformerFactory.Core().V1().ConfigMaps(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentConfigMapSyncs), ctx.Stop)
	return true, nil
}

func startPodController(ctx *ControllerContext) (bool, error) {
	go pod.NewpodController(
		ctx.InformerFactory.Core().V1().Pods(),
		ctx.ResponseChan,
		ctx.Namespace,
	).Run(int(ctx.Options.ConcurrentPodSyncs), ctx.Stop)
	return true, nil
}
