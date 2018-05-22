package deployment

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appv1 "k8s.io/client-go/informers/extensions/v1beta1"
	appv1_lister "k8s.io/client-go/listers/extensions/v1beta1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of service and pod changes.
	workerLoopPeriod  time.Duration
	lister            appv1_lister.DeploymentLister
	responseChan      chan<- *model.Response
	deploymentsSynced cache.InformerSynced
}

func NewDeploymentController(deploymentInformer appv1.DeploymentInformer, responseChan chan<- *model.Response) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "deployment"),
		workerLoopPeriod: time.Second,
		lister:           deploymentInformer.Lister(),
		responseChan:     responseChan,
	}

	deploymentInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueDeployment,
		UpdateFunc: func(old, new interface{}) {
			newDeployment := new.(*extensions.Deployment)
			oldDeployment := old.(*extensions.Deployment)
			if newDeployment.ResourceVersion == oldDeployment.ResourceVersion {
				return
			}
			c.enqueueDeployment(new)
		},
		DeleteFunc: c.enqueueDeployment,
	})
	c.deploymentsSynced = deploymentInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Pod controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced); !ok {
		glog.Error("failed to wait for caches to sync")
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")
}
func (c *controller) enqueueDeployment(obj interface{}) {
	var key string
	var err error
	if key, err = keyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

func (c *controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *controller) processNextWorkItem() bool {
	key, shutdown := c.queue.Get()

	if shutdown {
		return false
	}
	defer c.queue.Done(key)

	forget, err := c.syncHandler(key.(string))
	if err == nil {
		if forget {
			c.queue.Forget(key)
		}
		return true
	}

	runtime.HandleError(fmt.Errorf("error syncing '%s': %s", key, err.Error()))
	c.queue.AddRateLimited(key)

	return true
}

func (c *controller) syncHandler(key string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return true, nil
	}

	deployment, err := c.lister.Deployments(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.responseChan <- newDeploymentDelRep(name, namespace)
			runtime.HandleError(fmt.Errorf("pod '%s' in work queue no longer exists", key))
			return true, nil
		}
		return false, err
	}

	if deployment.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(deployment.Labels[model.ReleaseLabel], ":", deployment)
		c.responseChan <- newDeploymentRep(deployment)
	}
	return true, nil
}

func newDeploymentDelRep(name string, namespace string) *model.Response {

	return &model.Response{
		Key:  fmt.Sprintf("env:%s.Deployment:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newDeploymentRep(deployment *extensions.Deployment) *model.Response {
	payload, err := json.Marshal(deployment)
	release := deployment.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Deployment:%s", deployment.Namespace, release, deployment.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
