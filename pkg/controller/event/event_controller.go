package event

import (
	"fmt"
	"time"

	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	event_lister "k8s.io/client-go/listers/core/v1"
	event_informer "k8s.io/client-go/informers/core/v1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	clientset "k8s.io/client-go/kubernetes"
	"github.com/choerodon/choerodon-agent/pkg/model"
	"k8s.io/api/core/v1"
	"encoding/json"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           event_lister.EventLister
	responseChan     chan<- *model.Response
	eventsSynced      cache.InformerSynced
	namespace        string
	client           clientset.Interface
}

func NewEventController(eventInformer event_informer.EventInformer, responseChan chan<- *model.Response, namespace string, client clientset.Interface) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		workerLoopPeriod: time.Second,
		lister:           eventInformer.Lister(),
		responseChan:     responseChan,
		namespace: namespace,
		client:  client,

	}
	eventInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{

	})
	eventInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueEvent,
		UpdateFunc: func(old, new interface{}) {
			newEvnet := new.(*v1.Event)
			oldEvnet := old.(*v1.Event)
			if newEvnet.ResourceVersion == oldEvnet.ResourceVersion {
				return
			}
			c.enqueueEvent(new)
		},
		DeleteFunc: func(obj interface{}) {

		},
	})
	c.eventsSynced = eventInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Event controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.eventsSynced); !ok {
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
func (c *controller) enqueueEvent(obj interface{}) {
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

	event, err := c.lister.Events(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			glog.Info("event delete")
			return true, nil
		}
		return false, err
	}
	//if event.InvolvedObject.Kind != "Pod" {
	//	return true, nil
	//}
	//
	//_,err = c.client.CoreV1().Pods(namespace).Get(event.InvolvedObject.Name,meta_v1.GetOptions{})
	if event.InvolvedObject.Kind == "Job" {
		_,err = c.client.BatchV1().Jobs(namespace).Get(event.InvolvedObject.Name,meta_v1.GetOptions{})
		if err != nil {
			glog.Errorf("cant not get Job from event %s",event.InvolvedObject.Name)
			return true,nil
		}
		c.responseChan <- newEventRep(event)
	}
	if event.InvolvedObject.Kind == "Pod" {
		pod,err := c.client.CoreV1().Pods(namespace).Get(event.InvolvedObject.Name,meta_v1.GetOptions{})
		if err != nil {
			glog.Errorf("cant not get Pod from event %s",event.InvolvedObject.Name)
			return true,nil
		}
		if pod.Labels[model.ReleaseLabel] != "" {
			//实例pod产生的事件
			c.responseChan <- newInstanceEventRep(event, pod.Labels[model.ReleaseLabel])
			return true,nil

		}
		c.responseChan <- newEventRep(event)

	}

	return true, nil
}

func newEventRep(event *v1.Event) *model.Response {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.Event:%s", event.Namespace, event.Name),
		Type:    model.JobEvent,
		Payload: string(payload),
	}
}

func newInstanceEventRep(event *v1.Event, release string) *model.Response {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Event:%s", event.Namespace, release, event.Name),
		Type:    model.ReleasePodEvent,
		Payload: string(payload),
	}
}