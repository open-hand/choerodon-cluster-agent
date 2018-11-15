package c7nhelmrelease

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"strings"
	"time"

	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/runtime"
	"k8s.io/apimachinery/pkg/util/wait"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	typedcorev1 "k8s.io/client-go/kubernetes/typed/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/record"
	"k8s.io/client-go/util/workqueue"

	c7nv1alpha1 "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	chrclientset "github.com/choerodon/choerodon-cluster-agent/pkg/client/clientset/versioned"
	chrscheme "github.com/choerodon/choerodon-cluster-agent/pkg/client/clientset/versioned/scheme"
	chrinformers "github.com/choerodon/choerodon-cluster-agent/pkg/client/informers/externalversions/choerodon/v1alpha1"
	chrlisters "github.com/choerodon/choerodon-cluster-agent/pkg/client/listers/choerodon/v1alpha1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	modelhelm "github.com/choerodon/choerodon-cluster-agent/pkg/model/helm"
)

const (
	controllerAgentName = "c7nhelmrelease-controller"

	// SuccessSynced is used as part of the Event 'reason' when a C7NHelmRelease is synced
	SuccessSynced = "Synced"
	// MessageResourceSynced is the message used for an Event fired when a C7NHelmRelease
	// is synced successfully
	MessageResourceSynced = "C7NHelmRelease synced successfully"
)


type Controller struct {
	kubeClientset kubernetes.Interface
	chrClientset  chrclientset.Interface
	helmClient    helm.Client
	commandChan   chan<- *model.Packet
	chrLister     chrlisters.C7NHelmReleaseLister
	chrSync       cache.InformerSynced
	responseChan  chan<- *model.Packet
	workqueue     workqueue.RateLimitingInterface
	namespaces    *manager.Namespaces
	recorder      record.EventRecorder
}

func NewController(
	kubeClientset kubernetes.Interface,
	chrClientset chrclientset.Interface,
	chrInformer chrinformers.C7NHelmReleaseInformer,
	helmClient helm.Client,
	commandChan chan<- *model.Packet,
	namespaces *manager.Namespaces,
	responseChan chan<- *model.Packet) *Controller {

	chrscheme.AddToScheme(scheme.Scheme)
	glog.V(4).Info("Creating event broadcaster")
	eventBroadcaster := record.NewBroadcaster()
	eventBroadcaster.StartLogging(glog.V(1).Infof)
	eventBroadcaster.StartRecordingToSink(&typedcorev1.EventSinkImpl{Interface: kubeClientset.CoreV1().Events("")})
	recorder := eventBroadcaster.NewRecorder(scheme.Scheme, corev1.EventSource{Component: controllerAgentName})

	controller := &Controller{
		kubeClientset: kubeClientset,
		chrClientset:  chrClientset,
		chrLister:     chrInformer.Lister(),
		chrSync:       chrInformer.Informer().HasSynced,
		helmClient:    helmClient,
		commandChan:   commandChan,
		workqueue:     workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "C7NHelmReleases"),
		recorder:      recorder,
		namespaces:    namespaces,
		responseChan:  responseChan,
	}

	chrInformer.Informer().AddEventHandler(cache.ResourceEventHandlerFuncs{
		AddFunc: controller.enqueueChr,
		UpdateFunc: func(oldObj, newObj interface{}) {
			oldChr := oldObj.(*c7nv1alpha1.C7NHelmRelease)
			newChr := newObj.(*c7nv1alpha1.C7NHelmRelease)
			if oldChr.ResourceVersion == newChr.ResourceVersion {
				return
			}
			controller.enqueueChr(newObj)
		},
		DeleteFunc: controller.enqueueChr,
	})

	return controller
}

func (c *Controller) enqueueChr(obj interface{}) {
	var key string
	var err error
	if key, err = cache.MetaNamespaceKeyFunc(obj); err != nil {
		runtime.HandleError(err)
		return
	}
	c.workqueue.AddRateLimited(key)
}

func (c *Controller) Run(workers int, stopCh <-chan struct{}) {
	defer runtime.HandleCrash()
	defer c.workqueue.ShutDown()
	if ok := cache.WaitForCacheSync(stopCh, c.chrSync); !ok {
		glog.Fatal("failed to wait for caches to sync")
	}
	// Launch two workers to process C7NHelmRelease resources
	for i := 0; i < workers; i++ {
		go wait.Until(c.runWorker, time.Second, stopCh)
	}

	//go func() {
	//	refresh := time.NewTicker(20 * time.Second)
	//	for {
	//		select {
	//		case <-refresh.C:
	//			chrs, err := c.chrLister.C7NHelmReleases(c.namespace).List(labels.NewSelector())
	//			if err != nil {
	//				glog.Infof("list release error")
	//			}
	//			for _,chr := range chrs{
	//				rls, err := c.helmClient.GetReleaseContent(&modelhelm.GetReleaseContentRequest{ReleaseName: chr.Name})
	//				if err != nil {
	//					if !strings.Contains(err.Error(), helm.ErrReleaseNotFound(chr.Name).Error()) {
	//						if cmd := installHelmReleaseCmd(chr); cmd != nil {
	//							glog.Infof("release %s install in timer", chr.Name)
	//							c.commandChan <- cmd
	//						}
	//					} else {
	//						fmt.Errorf("get release content: %v", err)
	//					}
	//				} else {
	//					if chr.Spec.ChartName == rls.ChartName && chr.Spec.ChartVersion == rls.ChartVersion && chr.Spec.Values == rls.Config {
	//						glog.V(3).Infof("release %s chart、version、values not change in timer", rls.Name)
	//					} else if  string(rls.Status) != "DEPLOYED" || string(rls.Status) != "FAILED"  {
	//						glog.Infof("release statue : %s",rls.Status)
	//					} else if cmd := updateHelmReleaseCmd(chr); cmd != nil {
	//						glog.Infof("release %s upgrade", rls.Name)
	//						c.commandChan <- cmd
	//					}
	//				}
	//			}
	//		}
	//	}
	//}()

	//chrs, err := c.chrLister.C7NHelmReleases(c.namespace).List(labels.NewSelector())
	//if err != nil {
	//	glog.Fatal("can not list chrs!")
	//} else {
	//	var chrlist []string
	//	for _, chr := range chrs {
	//		chrlist = append(chrlist, chr.Name)
	//	}
	//	resourceList := &kubernetes2.ResourceList{
	//		Resources:    chrlist,
	//		ResourceType: "Release",
	//	}
	//	content, err := json.Marshal(resourceList)
	//	if err != nil {
	//		glog.Fatal("marshal pod list error")
	//	} else {
	//		response := &model.Packet{
	//			Key:     fmt.Sprintf("env:%s", c.namespace),
	//			Type:    model.ResourceSync,
	//			Payload: string(content),
	//		}
	//		c.responseChan <- response
	//	}
	//}
	<-stopCh
	glog.V(1).Info("Shutting down c7nhelmrelease workers")
}

// runWorker is a long-running function that will continually call the
// processNextWorkItem function in order to read and process a message on the
// workqueue.
func (c *Controller) runWorker() {
	for c.processNextWorkItem() {
	}
}

// processNextWorkItem will read a single work item off the workqueue and
// attempt to process it, by calling the syncHandler.
func (c *Controller) processNextWorkItem() bool {
	obj, shutdown := c.workqueue.Get()

	if shutdown {
		return false
	}

	// We wrap this block in a func so we can defer c.workqueue.Done.
	err := func(obj interface{}) error {
		// We call Done here so the workqueue knows we have finished
		// processing this item. We also must remember to call Forget if we
		// do not want this work item being re-queued. For example, we do
		// not call Forget if a transient error occurs, instead the item is
		// put back on the workqueue and attempted again after a back-off
		// period.
		defer c.workqueue.Done(obj)
		var key string
		var ok bool
		// We expect strings to come off the workqueue. These are of the
		// form namespace/name. We do this as the delayed nature of the
		// workqueue means the items in the informer cache may actually be
		// more up to date that when the item was initially put onto the
		// workqueue.
		if key, ok = obj.(string); !ok {
			// As the item in the workqueue is actually invalid, we call
			// Forget here else we'd go into a loop of attempting to
			// process a work item that is invalid.
			c.workqueue.Forget(obj)
			runtime.HandleError(fmt.Errorf("expected string in workqueue but got %#v", obj))
			return nil
		}
		// Run the syncHandler, passing it the namespace/name string of the
		// C7NHelmRelease resource to be synced.
		if err := c.syncHandler(key); err != nil {
			return fmt.Errorf("error syncing '%s': %s", key, err.Error())
		}
		// Finally, if no error occurs we Forget this item so it does not
		// get queued again until another change happens.
		c.workqueue.Forget(obj)
		return nil
	}(obj)

	if err != nil {
		runtime.HandleError(err)
		return true
	}

	return true
}

// syncHandler sync helm release according to action
func (c *Controller) syncHandler(key string) error {
	// Convert the namespace/name string into a distinct namespace and name
	namespace, name, err := cache.SplitMetaNamespaceKey(key)
	if err != nil {
		runtime.HandleError(fmt.Errorf("invalid resource key: %s", key))
		return nil
	}

	if !c.namespaces.Contain(namespace) {
		return nil
	}

	chr, err := c.chrLister.C7NHelmReleases(namespace).Get(name)

	if err != nil {
		// The C7NHelmRelease resource may no longer exist, in which case we stop processing.
		if errors.IsNotFound(err) {
			_,err := c.kubeClientset.Discovery().ServerResourcesForGroupVersion("choerodon.io/v1alpha1")
			if err != nil && errors.IsNotFound(err) {
				glog.Warningf("C7NHelmReleases CustomResourceDefinition been deleted!")
				return nil
			}
			runtime.HandleError(fmt.Errorf("C7NHelmReleases '%s' in work queue no longer exists", key))
			if cmd := deleteHelmReleaseCmd(namespace, name); cmd != nil {
				glog.Infof("release %s delete", name)
				c.commandChan <- cmd
			}
			return nil
		}
		return err
	}

	if chr.Annotations == nil || chr.Annotations[model.CommitLabel] == "" {
		return fmt.Errorf("c7nhelmrelease no commit annotations")
	}

	rls, err := c.helmClient.GetRelease(&modelhelm.GetReleaseContentRequest{ReleaseName: name})
	if err != nil {
		if !strings.Contains(err.Error(), helm.ErrReleaseNotFound(name).Error()) {
			if cmd := installHelmReleaseCmd(chr); cmd != nil {
				glog.Infof("release %s install", chr.Name)
				c.commandChan <- cmd
			}
		} else {
			c.responseChan <- newReleaseSyncFailRep(chr, "helm release query failed ,please check tiller server.")
			return fmt.Errorf("get release content: %v", err)
		}
	} else {
		if chr.Namespace != rls.Namespace {
			c.responseChan <- newReleaseSyncFailRep(chr, "release already in other namespace!")
			glog.Error("release already in other namespace!")
		}
		if chr.Spec.ChartName == rls.ChartName && chr.Spec.ChartVersion == rls.ChartVersion && chr.Spec.Values == rls.Config {
			glog.Infof("release %s chart、version、values not change", rls.Name)
			c.responseChan <- newReleaseSyncRep(chr)
			return nil
		}
		if cmd := updateHelmReleaseCmd(chr); cmd != nil {
			glog.Infof("release %s upgrade", rls.Name)
			c.commandChan <- cmd
		}
	}

	c.recorder.Event(chr, corev1.EventTypeNormal, SuccessSynced, MessageResourceSynced)

	return nil
}

func deleteHelmReleaseCmd(namespace, name string) *model.Packet {
	req := &modelhelm.DeleteReleaseRequest{ReleaseName: name}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", namespace, name),
		Type:    model.HelmReleaseDelete,
		Payload: string(reqBytes),
	}
}

func installHelmReleaseCmd(chr *c7nv1alpha1.C7NHelmRelease) *model.Packet {
	req := &modelhelm.InstallReleaseRequest{
		RepoURL:      chr.Spec.RepoURL,
		ChartName:    chr.Spec.ChartName,
		ChartVersion: chr.Spec.ChartVersion,
		Values:       chr.Spec.Values,
		ReleaseName:  chr.Name,
		Commit:       chr.Annotations[model.CommitLabel],
		Namespace:    chr.Namespace,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", chr.Namespace, chr.Name),
		Type:    model.HelmReleasePreInstall,
		Payload: string(reqBytes),
	}
}

func updateHelmReleaseCmd(chr *c7nv1alpha1.C7NHelmRelease) *model.Packet {
	req := &modelhelm.UpgradeReleaseRequest{
		RepoURL:      chr.Spec.RepoURL,
		ChartName:    chr.Spec.ChartName,
		ChartVersion: chr.Spec.ChartVersion,
		Values:       chr.Spec.Values,
		ReleaseName:  chr.Name,
		Commit:       chr.Annotations[model.CommitLabel],
		Namespace:    chr.Namespace,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", chr.Namespace, chr.Name),
		Type:    model.HelmReleasePreUpgrade,
		Payload: string(reqBytes),
	}
}

func newReleaseSyncRep(chr *c7nv1alpha1.C7NHelmRelease) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.release:%s.commit:%s", chr.Namespace, chr.Name, chr.Annotations[model.CommitLabel]),
		Type: model.HelmReleaseSynced,
	}
}

func newReleaseSyncFailRep(chr *c7nv1alpha1.C7NHelmRelease, msg string) *model.Packet {
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.commit:%s", chr.Namespace, chr.Name, chr.Annotations[model.CommitLabel]),
		Type:    model.HelmReleaseSyncedFailed,
		Payload: msg,
	}
}
