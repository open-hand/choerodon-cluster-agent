package pod

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1_informer "k8s.io/client-go/informers/core/v1"
	v1_lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/choerodon/choerodon-agent/pkg/model"
	"github.com/choerodon/choerodon-agent/pkg/model/kubernetes"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_lister.PodLister
	responseChan     chan<- *model.Response
	podsSynced       cache.InformerSynced
	namespace        string
}

func NewpodController(podInformer v1_informer.PodInformer, responseChan chan<- *model.Response, namespace string) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		workerLoopPeriod: time.Second,
		lister:           podInformer.Lister(),
		responseChan:     responseChan,
		namespace:        namespace,
	}

	podInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueuepod,
		UpdateFunc: func(old, new interface{}) {
			newpod := new.(*v1.Pod)
			oldpod := old.(*v1.Pod)
			if newpod.ResourceVersion == oldpod.ResourceVersion {
				return
			}
			c.enqueuepod(new)
		},
		DeleteFunc: c.enqueuepod,
	})
	c.podsSynced = podInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting pod controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.podsSynced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}

	pods, err := c.lister.Pods(c.namespace).List(labels.NewSelector())
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
			response := &model.Response{
				Key:     fmt.Sprintf("env:%s", c.namespace),
				Type:    model.ResourceSync,
				Payload: string(content),
			}
			c.responseChan <- response
		}
	}

	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	glog.Info("Started pod workers")
	<-stopCh
	glog.Info("Shutting down pod workers")
}
func (c *controller) enqueuepod(obj interface{}) {
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

	pod, err := c.lister.Pods(namespace).Get(name)
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

func newPodDelRep(name string, namespace string) *model.Response {
	return &model.Response{
		Key:  fmt.Sprintf("env:%s.Pod:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newPodRep(pod *v1.Pod) *model.Response {
	payload, err := json.Marshal(pod)
	release := pod.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Pod:%s", pod.Namespace, release, pod.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func getObjType(kind string) string {
	index := strings.LastIndex(kind, ".")
	return kind[index+1:]
}
