package secret

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
	v1_listers "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of secret and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_listers.SecretLister
	responseChan     chan<- *model.Response
	secretsSynced    cache.InformerSynced
}

func NewSecretController(secretInformer v1_informer.SecretInformer, responseChan chan<- *model.Response) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "endpoint"),
		workerLoopPeriod: time.Second,
		lister:           secretInformer.Lister(),
		responseChan:     responseChan,
	}

	secretInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueSecret,
		UpdateFunc: func(old, new interface{}) {
			newSecret := new.(*v1.Secret)
			oldSecret := old.(*v1.Secret)
			if newSecret.ResourceVersion == oldSecret.ResourceVersion {
				return
			}
			c.enqueueSecret(new)
		},
		DeleteFunc: c.enqueueSecret,
	})
	c.secretsSynced = secretInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Pod controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.secretsSynced); !ok {
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
func (c *controller) enqueueSecret(obj interface{}) {
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

	secret, err := c.lister.Secrets(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {

			c.responseChan <- newsecretDelRep(name, namespace)
			runtime.HandleError(fmt.Errorf("pod '%s' in work queue no longer exists", key))
			return true, nil
		}
		return false, err
	}

	if secret.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(secret.Labels[model.ReleaseLabel], ":", secret)
		c.responseChan <- newsecretRep(secret)
	}
	return true, nil
}

func newsecretDelRep(name string, namespace string) *model.Response {

	return &model.Response{
		Key:  fmt.Sprintf("env:%s.Secret:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newsecretRep(secret *v1.Secret) *model.Response {
	payload, err := json.Marshal(secret)
	release := secret.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Secret:%s", secret.Namespace, release, secret.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
