package event

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"time"

	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	event_informer "k8s.io/client-go/informers/core/v1"
	clientset "k8s.io/client-go/kubernetes"
	event_lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"strings"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           event_lister.EventLister
	responseChan     chan<- *model.Packet
	eventsSynced     cache.InformerSynced
	client           clientset.Interface
	namespaces       *manager.Namespaces
	platformCode     string
}

func NewEventController(eventInformer event_informer.EventInformer, responseChan chan<- *model.Packet, namespaces *manager.Namespaces, client clientset.Interface, platformCode string) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		workerLoopPeriod: time.Second,
		lister:           eventInformer.Lister(),
		responseChan:     responseChan,
		namespaces:       namespaces,
		client:           client,
		platformCode:     platformCode,
	}
	eventInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{})
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
	if ok := cache.WaitForCacheSync(stopCh, c.eventsSynced); !ok {
		glog.Error("failed to wait for caches to sync")
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}
	<-stopCh
	glog.V(1).Info("Shutting down workers")
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

	if !c.namespaces.Contain(namespace) {
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
		job, err := c.client.BatchV1().Jobs(namespace).Get(event.InvolvedObject.Name, meta_v1.GetOptions{})
		if err != nil {
			return true, nil
		}
		if job.Labels[model.TestLabel] == "" {
			c.responseChan <- newEventRep(event)
		}
	}
	if event.InvolvedObject.Kind == "Pod" {
		pod, err := c.client.CoreV1().Pods(namespace).Get(event.InvolvedObject.Name, meta_v1.GetOptions{})
		if err != nil {
			return true, nil
		}
		if pod.Labels[model.ReleaseLabel] != "" && pod.Labels[model.TestLabel] == "" {
			//实例pod产生的事件
			c.responseChan <- newInstanceEventRep(event, pod.Labels[model.ReleaseLabel])
			return true, nil

		} else if pod.Labels[model.TestLabel] == c.platformCode	{
			//测试pod事件
			c.responseChan <- newTestPodEventRep(event, pod.Labels[model.ReleaseLabel], pod.Labels[model.TestLabel])
		}

	}

	if event.InvolvedObject.Kind == "Certificate" && strings.Contains(event.Reason, "Err") {
		if len(event.Message) > 43 {
			glog.Warningf("Certificate err event reason not contain commit")
			c.responseChan <- newCertFailed(event)
		}
	}

	return true, nil
}

func newEventRep(event *v1.Event) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Event:%s", event.Namespace, event.Name),
		Type:    model.JobEvent,
		Payload: string(payload),
	}
}

func newInstanceEventRep(event *v1.Event, release string) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Event:%s", event.Namespace, release, event.Name),
		Type:    model.ReleasePodEvent,
		Payload: string(payload),
	}
}

func newTestPodEventRep(event *v1.Event, release string, label string) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.label:%s", event.Namespace, release, label),
		Type:    model.TestPodEvent,
		Payload: string(payload),
	}
}

func newCertFailed(event *v1.Event) *model.Packet {
	commit := event.Message[1:41]
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Cert:%s.commit:%s", event.Namespace, event.InvolvedObject.Name, commit),
		Type:    model.Cert_Faild,
		Payload: event.Message[42:],
	}
}
