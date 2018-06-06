package job

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	v1_informer "k8s.io/client-go/informers/batch/v1"
	v1_lister "k8s.io/client-go/listers/batch/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"

	"github.com/choerodon/choerodon-agent/pkg/kube"
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
	lister           v1_lister.JobLister
	responseChan     chan<- *model.Response
	jobSynced        cache.InformerSynced
	kubeClient       kube.Client
	namespace		  string
}

func NewJobController(jobInformer v1_informer.JobInformer, client kube.Client, responseChan chan<- *model.Response,namespace string) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "job"),
		workerLoopPeriod: time.Second,
		lister:           jobInformer.Lister(),
		responseChan:     responseChan,
		kubeClient:       client,
		namespace:namespace,
	}

	jobInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: c.enqueueJob,
		UpdateFunc: func(old, new interface{}) {
			newJob := new.(*v1.Job)
			oldJob := old.(*v1.Job)
			if newJob.ResourceVersion == oldJob.ResourceVersion {
				return
			}
			c.enqueueJob(new)
		},
		DeleteFunc: c.enqueueJob,
	})
	c.jobSynced = jobInformer.Informer().HasSynced
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.queue.ShutDown()

	// Start the informer factories to begin populating the informer caches
	glog.Info("Starting Pod controller")

	// Wait for the caches to be synced before starting workers
	glog.Info("Waiting for informer caches to sync")
	if ok := cache.WaitForCacheSync(stopCh, c.jobSynced); !ok {
		glog.Error("failed to wait for caches to sync")
	}

	resources,err := c.lister.Jobs(c.namespace).List(labels.NewSelector())
	if err != nil {
		glog.Error("failed list jobs")
	}else {
		var resourceList []string
		for _,resource := range resources {
			if resource.Labels[model.ReleaseLabel] != ""{
				resourceList = append(resourceList, resource.GetName())
			}
		}
		resourceListResp := &kubernetes.ResourceList{
			Resources: resourceList,
			ResourceType: "Job",
		}
		content,err := json.Marshal(resourceListResp)
		if err!= nil {
			glog.Error("marshal job list error")
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
func (c *controller) enqueueJob(obj interface{}) {
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

	job, err := c.lister.Jobs(namespace).Get(name)
	if err != nil {
		if errors.IsNotFound(err) {
			if job.Labels[model.ReleaseLabel] != "" {
				c.responseChan <- newJobDelRep(name, namespace)
			}
			runtime.HandleError(fmt.Errorf("pod '%s' in work queue no longer exists", key))
			return true, nil
		}
		return false, err
	}

	if job.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(job.Labels[model.ReleaseLabel], ":", job)
		c.responseChan <- newJobRep(job)
		if IsJobFinished(job) {
			jobLogs, err := c.kubeClient.LogsForJob(namespace, job.Name)
			if err != nil {
				glog.Error("get job log error ", err)
			} else if strings.TrimSpace(jobLogs) != "" {
				c.responseChan <- newJobLogRep(job.Name, job.Labels[model.ReleaseLabel], jobLogs, namespace)
			}
			err = c.kubeClient.DeleteJob(namespace, job.Name)
			if err != nil {
				glog.Error("delete job error", err)
			}
		}

	}
	return true, nil
}

func newJobDelRep(name string, namespace string) *model.Response {

	return &model.Response{
		Key:  fmt.Sprintf("env:%s.Job:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newJobLogRep(name string, release string, jobLogs string, namespace string) *model.Response {
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Job:%s", namespace, release, name),
		Type:    model.HelmReleaseHookGetLogs,
		Payload: jobLogs,
	}
}

func newJobRep(job *v1.Job) *model.Response {
	payload, err := json.Marshal(job)
	release := job.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Response{
		Key:     fmt.Sprintf("env:%s.release:%s.Job:%s", job.Namespace, release, job.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func IsJobFinished(j *v1.Job) bool {
	for _, c := range j.Status.Conditions {
		if (c.Type == v1.JobComplete || c.Type == v1.JobFailed) && c.Status == "True" {
			return true
		}
	}
	return false
}

func IsJobCompleted(j *v1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == v1.JobComplete && c.Status == "True" {
			return true
		}
	}
	return false
}

func IsJobFailed(j *v1.Job) bool {
	for _, c := range j.Status.Conditions {
		if c.Type == v1.JobFailed && c.Status == "True" {
			return true
		}
	}
	return false
}
