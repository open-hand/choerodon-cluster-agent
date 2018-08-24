package replicaset

import (
	"encoding/json"
	"fmt"
	"time"

	"github.com/golang/glog"
	extensions "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	appv1 "k8s.io/client-go/informers/extensions/v1beta1"
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
	// workerLoopPeriod is the time between worker runs. The workers process the queue of service and pod changes.
	workerLoopPeriod  time.Duration
	replicsetInformer appv1.ReplicaSetInformer
	responseChan      chan<- *model.Response
	replicasetsSynced cache.InformerSynced
	namespace         string
}

func NewReplicaSetController(replicasetInformer appv1.ReplicaSetInformer, responseChan chan<- *model.Response, namespace string) *controller {

	c := &controller{
		queue:             workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "replicaset"),
		workerLoopPeriod:  time.Second,
		replicsetInformer: replicasetInformer,
		responseChan:      responseChan,
		namespace:         namespace,
	}

	replicasetInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueReplicaSet,
		UpdateFunc: func(old, new interface{}) {
			newReplicaSet := new.(*extensions.ReplicaSet)
			oldReplicaSet := old.(*extensions.ReplicaSet)
			if newReplicaSet.ResourceVersion == oldReplicaSet.ResourceVersion {
				return
			}
			c.enqueueReplicaSet(new)
		},
		DeleteFunc: c.enqueueReplicaSet,
	})
	c.replicasetsSynced = replicasetInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting replicaset controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.replicasetsSynced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}

	resources, err := c.replicsetInformer.Lister().List(labels.NewSelector())
	if err != nil {
		glog.Fatal("failed list replicaset")
	} else {
		var resourceList []string
		for _, resource := range resources {
			if resource.Labels[model.ReleaseLabel] != "" {
				resourceList = append(resourceList, resource.GetName())
			}
		}
		resourceListResp := &kubernetes.ResourceList{
			Resources:    resourceList,
			ResourceType: "ReplicaSet",
		}
		content, err := json.Marshal(resourceListResp)
		if err != nil {
			glog.Fatal("marshal pod replicaSet error")
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

	glog.Info("Started replicaset workers")
	<-stopCh
	glog.Info("Shutting down replicaset workers")
}
func (c *controller) enqueueReplicaSet(obj interface{}) {
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

	replicaset, err := c.replicsetInformer.Lister().ReplicaSets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.responseChan <- newReplicaSetDelRep(name, namespace)
			glog.Warningf("Rs '%s' in work queue no longer exists", key)
			return true, nil
		}
		return false, err
	}

	if replicaset.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(replicaset.Labels[model.ReleaseLabel], ":", replicaset)
		c.responseChan <- newReplicaSetRep(replicaset)
	}
	return true, nil
}

func newReplicaSetDelRep(name string, namespace string) *model.Response {

	return &model.Response{
		Key:  fmt.Sprintf("env:%s.ReplicaSet:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newReplicaSetRep(replicaset *extensions.ReplicaSet) *model.Response {
	payload, err := json.Marshal(replicaset)
	release := replicaset.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.ReplicaSet:%s", replicaset.Namespace, release, replicaset.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
