package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"time"

	"github.com/golang/glog"
	kubeinformers "k8s.io/client-go/informers"
	clientset "k8s.io/client-go/kubernetes"

	"github.com/choerodon/choerodon-cluster-agent/controller/configmap"
	"github.com/choerodon/choerodon-cluster-agent/controller/deployment"
	"github.com/choerodon/choerodon-cluster-agent/controller/endpoint"
	"github.com/choerodon/choerodon-cluster-agent/controller/ingress"
	"github.com/choerodon/choerodon-cluster-agent/controller/job"
	"github.com/choerodon/choerodon-cluster-agent/controller/pod"
	"github.com/choerodon/choerodon-cluster-agent/controller/replicaset"
	"github.com/choerodon/choerodon-cluster-agent/controller/secret"
	"github.com/choerodon/choerodon-cluster-agent/controller/service"

	"github.com/choerodon/choerodon-cluster-agent/controller/c7nhelmrelease"
	"github.com/choerodon/choerodon-cluster-agent/controller/event"
	chrclientset "github.com/choerodon/choerodon-cluster-agent/pkg/client/clientset/versioned"
	c7ninformers "github.com/choerodon/choerodon-cluster-agent/pkg/client/informers/externalversions"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
)

type InitFunc func(ctx *ControllerContext) (bool, error)

const workers int  = 1

type ControllerContext struct {
	kubeInformer  kubeinformers.SharedInformerFactory
	kubeClientset clientset.Interface
	c7nClientset  chrclientset.Interface
	c7nInformer   c7ninformers.SharedInformerFactory
	kubeClient    kube.Client
	helmClient    helm.Client
	stop          <-chan struct{}
	chans         *manager.CRChan
	Namespaces    *manager.Namespaces
}

func CreateControllerContext(
	kubeClientset clientset.Interface,
	c7nClientset chrclientset.Interface,
	kubeClient kube.Client,
	helmClient helm.Client,
	stop <-chan struct{},
	chans *manager.CRChan,
	Namespaces *manager.Namespaces) *ControllerContext {
	kubeInformer := kubeinformers.NewSharedInformerFactory(kubeClientset, time.Second*30)
	c7nInformer := c7ninformers.NewSharedInformerFactory(c7nClientset, time.Second*30)

	ctx := &ControllerContext{
		kubeInformer:  kubeInformer,
		kubeClientset: kubeClientset,
		c7nClientset:  c7nClientset,
		c7nInformer:   c7nInformer,
		kubeClient:    kubeClient,
		helmClient:    helmClient,
		stop:          stop,
		Namespaces:    Namespaces,
		chans:         chans,
	}
	return ctx
}



func (ctx *ControllerContext) StartControllers() error {
	controllers := map[string]InitFunc{}
	controllers["endpoint"] = startEndpointController
	controllers["deployment"] = startDeploymentController
	controllers["job"] = startJobController
	controllers["service"] = startServiceController
	controllers["secret"] = startSecretController
	//controllers["configmap"] = startConfigMapController
	controllers["ingress"] = startIngressController
	controllers["replicaset"] = startReplicaSetController
	controllers["pod"] = startPodController
	controllers["event"] = startEventController
	//controllers["c7nhelmrelease"] = startC7NHelmReleaseController
	glog.V(1).Infof("Starting controllers")
	for controllerName, initFn := range controllers {
		started, err := initFn(ctx)
		if err != nil {
			glog.Errorf("Error starting %q", controllerName)
			return err
		}
		if !started {
			glog.Warningf("Skipping %q", controllerName)
			continue
		}
	}
	ctx.kubeInformer.Start(ctx.stop)
	ctx.c7nInformer.Start(ctx.stop)
	return nil
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

func startDeploymentController(ctx *ControllerContext) (bool, error) {
	go deployment.NewDeploymentController(
		ctx.kubeInformer.Extensions().V1beta1().Deployments(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startIngressController(ctx *ControllerContext) (bool, error) {
	go ingress.NewIngressController(
		ctx.kubeInformer.Extensions().V1beta1().Ingresses(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startReplicaSetController(ctx *ControllerContext) (bool, error) {
	go replicaset.NewReplicaSetController(
		ctx.kubeInformer.Extensions().V1beta1().ReplicaSets(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startJobController(ctx *ControllerContext) (bool, error) {
	go job.NewJobController(
		ctx.kubeInformer.Batch().V1().Jobs(),
		ctx.kubeClient,
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startServiceController(ctx *ControllerContext) (bool, error) {
	go service.NewserviceController(
		ctx.kubeInformer.Core().V1().Services(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startSecretController(ctx *ControllerContext) (bool, error) {
	go secret.NewSecretController(
		ctx.kubeInformer.Core().V1().Secrets(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startConfigMapController(ctx *ControllerContext) (bool, error) {
	go configMap.NewconfigMapController(
		ctx.kubeInformer.Core().V1().ConfigMaps(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startPodController(ctx *ControllerContext) (bool, error) {
	go pod.NewpodController(
		ctx.kubeInformer.Core().V1().Pods(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
	).Run(workers, ctx.stop)
	return true, nil
}

func startC7NHelmReleaseController(ctx *ControllerContext) (bool, error) {
	go c7nhelmrelease.NewController(
		ctx.kubeClientset,
		ctx.c7nClientset,
		ctx.c7nInformer.Choerodon().V1alpha1().C7NHelmReleases(),
		ctx.helmClient,
		ctx.chans.CommandChan,
		ctx.Namespaces,
		ctx.chans.ResponseChan,
	).Run(workers, ctx.stop)
	return true, nil
}

func startEventController(ctx *ControllerContext) (bool, error) {
	go event.NewEventController(
		ctx.kubeInformer.Core().V1().Events(),
		ctx.chans.ResponseChan,
		ctx.Namespaces,
		ctx.kubeClientset,
	).Run(workers, ctx.stop)
	return true, nil
}
