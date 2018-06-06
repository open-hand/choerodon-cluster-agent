package service

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1_informer "k8s.io/client-go/informers/core/v1"
	v1_lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/choerodon/choerodon-agent/pkg/model"
	"k8s.io/apimachinery/pkg/labels"
	"github.com/choerodon/choerodon-agent/pkg/model/kubernetes"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of service and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_lister.ServiceLister
	responseChan     chan<- *model.Response
	servicesSynced   cache.InformerSynced
	namespace		  string
}

func NewserviceController(serviceInformer v1_informer.ServiceInformer, responseChan chan<- *model.Response,namespace string) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "service"),
		workerLoopPeriod: time.Second,
		lister:           serviceInformer.Lister(),
		responseChan:     responseChan,
		namespace:namespace,
	}

	serviceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueservice,
		UpdateFunc: func(old, new interface{}) {
			newservice := new.(*v1.Service)
			oldservice := old.(*v1.Service)
			if newservice.ResourceVersion == oldservice.ResourceVersion {
				return
			}
			c.enqueueservice(new)
		},
		DeleteFunc: c.enqueueservice,
	})
	c.servicesSynced = serviceInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Pod controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.servicesSynced); !ok {
		glog.Error("failed to wait for caches to sync")
	}

	resources,err := c.lister.Services(c.namespace).List(labels.NewSelector())
	if err != nil {
		glog.Error("failed list service")
	}else {
		var resourceList []string
		for _,resource := range resources {
			if resource.Labels[model.NetworkLabel] != "" {
				resourceList = append(resourceList, resource.GetName())
			}
		}
		resourceListResp := &kubernetes.ResourceList{
			Resources: resourceList,
			ResourceType: "Service",
		}
		content,err := json.Marshal(resourceListResp)
		if err!= nil {
			glog.Error("marshal service list error")
		}else {
			response := &model.Response{
				Key: fmt.Sprintf("env:%s", c.namespace),
				Type: model.ResourceSync,
				Payload: string(content),
			}
			c.responseChan <- response

		}
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started workers")
	<-stopCh
	glog.Info("Shutting down workers")
}
func (c *controller) enqueueservice(obj interface{}) {
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

	service, err := c.lister.Services(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			if service.Labels[model.NetworkLabel] != "" {
				c.responseChan <- newServiceDelRep(name, namespace)
			}
			runtime.HandleError(fmt.Errorf("service '%s' in work queue no longer exists", key))
			return true, nil
		}
		return false, err
	}

	if service.Labels[model.NetworkLabel] != "" {
		glog.V(2).Info(service.Labels[model.ReleaseLabel], ":", service)
		c.responseChan <- newServiceRep(service)
	}

	return true, nil
}

func newServiceDelRep(name string, namespace string) *model.Response {

	return &model.Response{
		Key:  fmt.Sprintf("env:%s.Service:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newServiceRep(service *v1.Service) *model.Response {
	payload, err := json.Marshal(service)
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.Service:%s", service.Namespace, service.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
