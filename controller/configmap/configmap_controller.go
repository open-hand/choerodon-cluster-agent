package configMap

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
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

	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of configMap and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_lister.ConfigMapLister
	responseChan     chan<- *model.Packet
	configMapsSynced cache.InformerSynced
	namespaces       *manager.Namespaces
}

func NewconfigMapController(configMapInformer v1_informer.ConfigMapInformer, responseChan chan<- *model.Packet, namespaces *manager.Namespaces) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "cofigmap"),
		workerLoopPeriod: time.Second,
		responseChan:     responseChan,
		lister:           configMapInformer.Lister(),
		namespaces:       namespaces,
	}

	configMapInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueconfigMap,
		UpdateFunc: func(old, new interface{}) {
			newconfigMap := new.(*v1.ConfigMap)
			oldconfigMap := old.(*v1.ConfigMap)
			if newconfigMap.ResourceVersion == oldconfigMap.ResourceVersion {
				return
			}
			c.enqueueconfigMap(new)
		},
		DeleteFunc: c.enqueueconfigMap,
	})
	c.configMapsSynced = configMapInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()
	if ok := cache.WaitForCacheSync(stopCh, c.configMapsSynced); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}
	// Launch two workers to process Foo resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	<-stopCh
	glog.V(1).Info("Shutting down configmap workers")
}
func (c *controller) enqueueconfigMap(obj interface{}) {
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

	configMap, err := c.lister.ConfigMaps(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			c.responseChan <- newConfigMapDelRep(name, namespace)
			glog.Warningf("configmap '%s' in work queue no longer exists", key)
			return true, nil
		}
		return false, err
	}

	if configMap.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(configMap.Labels[model.ReleaseLabel], ":", configMap)
		c.responseChan <- newconfigMapRep(configMap)
	} else if configMap.Annotations[model.MicroServiceConfig] != "" {
		c.responseChan <- newConfigConfigMapRep(configMap)
	} else if configMap.Labels[model.AgentVersionLabel] != "" {
		glog.V(2).Info(configMap.Labels[model.ReleaseLabel], ":", configMap)
		c.responseChan <- newRepoConfigMapRep(configMap)
	}

	return true, nil
}

func newConfigMapDelRep(name string, namespace string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.configMap:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newconfigMapRep(configMap *v1.ConfigMap) *model.Packet {
	payload, err := json.Marshal(configMap)
	release := configMap.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.configMap:%s", configMap.Namespace, release, configMap.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func newRepoConfigMapRep(configMap *v1.ConfigMap) *model.Packet {
	payload, err := json.Marshal(configMap)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.configMap:%s", configMap.Namespace, configMap.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func newConfigConfigMapRep(configMap *v1.ConfigMap) *model.Packet {
	payload, err := json.Marshal(configMap)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.configMap:%s", configMap.Namespace, configMap.Name),
		Type:    model.ConfigUpdate,
		Payload: string(payload),
	}
}
