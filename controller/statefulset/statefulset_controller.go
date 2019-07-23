package statefulset

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/golang/glog"
	"k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta2"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1_informer "k8s.io/client-go/informers/apps/v1"
	beta2_informer "k8s.io/client-go/informers/apps/v1beta2"
	v1_lister "k8s.io/client-go/listers/apps/v1"
	beta2_lister "k8s.io/client-go/listers/apps/v1beta2"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_lister.StatefulSetLister
	responseChan     chan<- *model.Packet
	synced           cache.InformerSynced
	namespaces       *namespace.Namespaces
}

func NewStatefulSetController(statefulSetInformer v1_informer.StatefulSetInformer, responseChan chan<- *model.Packet, namespaces *namespace.Namespaces) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "statefulSet"),
		workerLoopPeriod: time.Second,
		lister:           statefulSetInformer.Lister(),
		responseChan:     responseChan,
		namespaces:       namespaces,
	}

	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueStatefulSet,
		UpdateFunc: func(old, new interface{}) {
			newpod := new.(*v1.StatefulSet)
			oldpod := old.(*v1.StatefulSet)
			if newpod.ResourceVersion == oldpod.ResourceVersion {
				return
			}
			c.enqueueStatefulSet(new)
		},
		DeleteFunc: c.enqueueStatefulSet,
	})
	c.synced = statefulSetInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	if ok := cache.WaitForCacheSync(stopCh, c.synced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}

	namespaces := c.namespaces.GetAll()
	for _, ns := range namespaces {
		pods, err := c.lister.StatefulSets(ns).List(labels.NewSelector())
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
				c.responseChan <- response
			}
		}
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.V(1).Info("Shutting down pod workers")
}
func (c *controller) enqueueStatefulSet(obj interface{}) {
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

	pod, err := c.lister.StatefulSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.responseChan <- newPodDelRep(name, namespace)
			glog.Warningf("pod '%s' in work queue no longer exists", key)
			return true, nil
		}
		return false, err
	}

	if pod.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(pod.Labels[model.ReleaseLabel], ":", pod)
		c.responseChan <- newPodRep(pod)
	}
	return true, nil
}

func newPodDelRep(name string, namespace string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.StatefulSet:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newPodRep(pod *v1.StatefulSet) *model.Packet {
	payload, err := json.Marshal(pod)
	release := pod.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.StatefulSet:%s", pod.Namespace, release, pod.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

type beta2Controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           beta2_lister.StatefulSetLister
	responseChan     chan<- *model.Packet
	synced           cache.InformerSynced
	namespaces       *namespace.Namespaces
}

func NewBeta2StatefulSetController(statefulSetInformer beta2_informer.StatefulSetInformer, responseChan chan<- *model.Packet, namespaces *namespace.Namespaces) *beta2Controller {

	c := &beta2Controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "statefulSet"),
		workerLoopPeriod: time.Second,
		lister:           statefulSetInformer.Lister(),
		responseChan:     responseChan,
		namespaces:       namespaces,
	}

	statefulSetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueStatefulSet,
		UpdateFunc: func(old, new interface{}) {
			newpod := new.(*v1beta2.StatefulSet)
			oldpod := old.(*v1beta2.StatefulSet)
			if newpod.ResourceVersion == oldpod.ResourceVersion {
				return
			}
			c.enqueueStatefulSet(new)
		},
		DeleteFunc: c.enqueueStatefulSet,
	})
	c.synced = statefulSetInformer.Informer().HasSynced
	return c
}

func (c *beta2Controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	if ok := cache.WaitForCacheSync(stopCh, c.synced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}

	namespaces := c.namespaces.GetAll()
	for _, ns := range namespaces {
		pods, err := c.lister.StatefulSets(ns).List(labels.NewSelector())
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
				c.responseChan <- response
			}
		}
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.V(1).Info("Shutting down pod workers")
}
func (c *beta2Controller) enqueueStatefulSet(obj interface{}) {
	var key string
	var err error
	if key, err = keyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.queue.AddRateLimited(key)
}

func (c *beta2Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

func (c *beta2Controller) processNextWorkItem() bool {
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

func (c *beta2Controller) syncHandler(key string) (bool, error) {
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return true, nil
	}

	if !c.namespaces.Contain(namespace) {
		return true, nil
	}

	pod, err := c.lister.StatefulSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.responseChan <- newPodDelRep(name, namespace)
			glog.Warningf("pod '%s' in work queue no longer exists", key)
			return true, nil
		}
		return false, err
	}

	if pod.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(pod.Labels[model.ReleaseLabel], ":", pod)
		c.responseChan <- newBeta2Rep(pod)
	}
	return true, nil
}

func newBeta2Rep(pod *v1beta2.StatefulSet) *model.Packet {
	payload, err := json.Marshal(pod)
	release := pod.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.StatefulSet:%s", pod.Namespace, release, pod.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
