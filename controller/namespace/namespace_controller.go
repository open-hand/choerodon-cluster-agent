package namespace

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	informer_v1 "k8s.io/client-go/informers/core/v1"
	"k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
)



var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
	flag = false
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of service and pod changes.
	workerLoopPeriod  time.Duration
	lister			  v1.NamespaceLister
	responseChan      chan<- *model.Packet
	deploymentsSynced cache.InformerSynced
}

func NewNamespaceController(namespaceInformer informer_v1.NamespaceInformer, responseChan chan<- *model.Packet) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "deployment"),
		workerLoopPeriod: time.Second,
		lister:           namespaceInformer.Lister(),
		responseChan:     responseChan,
	}

	namespaceInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueNamespace,

		DeleteFunc: c.enqueueNamespace,
	})
	c.deploymentsSynced = namespaceInformer.Informer().HasSynced

	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	if ok := cache.WaitForCacheSync(stopCh, c.deploymentsSynced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}

	//namespaces,err := c.lister.List(labels.Everything())
	//nsList := []string{}
	//for _,ns := range namespaces {
	//	nsList = append(nsList, ns.GetName())
	//}
	//if err != nil {
	//	glog.Errorf("namespace controller list namespace error: %v", err)
	//} else {
	//	c.responseChan <- newNamesapcesRep(nsList)
	//
	//}
	flag = true


	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}


	<-stopCh
	glog.Info("Shutting down deployment workers")
}
func (c *controller) enqueueNamespace(obj interface{}) {
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
	//_, name, err := cache.SplitMetaNamespaceKey(key)
	//if err != nil {
	//	runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
	//	return true, nil
	//}



	namespaces,err := c.lister.List(labels.Everything())
	if err != nil {
		glog.Errorf("namespace controller list namespace error: %v", err)
	}
	nsList := []string{}
	for _,ns := range namespaces {
		nsList = append(nsList, ns.GetName())
	}
	c.responseChan <- newNamesapcesRep(nsList)

	return true, nil
}



func newNamesapcesRep(namespace []string) *model.Packet {
	payload, err := json.Marshal(namespace)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("none:none"),
		Type:    model.NamespaceUpdate,
		Payload: string(payload),
	}
}