package configmap

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_configmap")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new ConfigMap Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileConfigMap{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("configmap-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource ConfigMap
	err = c.Watch(&source.Kind{Type: &corev1.ConfigMap{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileConfigMap implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileConfigMap{}

// ReconcileConfigMap reconciles a ConfigMap object
type ReconcileConfigMap struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a ConfigMap object and makes changes based on the state read
// and what is in the ConfigMap.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileConfigMap) Reconcile(request reconcile.Request) (reconcile.Result, error) {

	// do not logging because some app use cm to leader select
	//reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	//reqLogger.Info("Reconciling ConfigMap")

	responseChan := r.args.CrChan.ResponseChan

	if !r.args.Namespaces.Contain(request.Namespace) {
		return reconcile.Result{}, nil
	}

	// Fetch the ConfigMap instance
	instance := &corev1.ConfigMap{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {

			responseChan <- newConfigMapDelRep(request.Name, request.Namespace)
			glog.Warningf("configmap '%s' in work queue no longer exists", request.Name)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if strings.Contains(instance.Labels["release"], "prometheus-operator") {
		return reconcile.Result{}, nil
	}

	if instance.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(instance.Labels[model.ReleaseLabel], ":", instance)
		responseChan <- newconfigMapRep(instance)
	} else if instance.Annotations[model.MicroServiceConfig] != "" {
		responseChan <- newConfigConfigMapRep(instance)
	} else if instance.Labels[model.AgentVersionLabel] != "" {
		glog.V(2).Info(instance.Labels[model.ReleaseLabel], ":", instance)
		responseChan <- newRepoConfigMapRep(instance)
	}

	return reconcile.Result{}, nil
}

func newConfigMapDelRep(name string, namespace string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.configMap:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newconfigMapRep(configMap *corev1.ConfigMap) *model.Packet {
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

func newRepoConfigMapRep(configMap *corev1.ConfigMap) *model.Packet {
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

func newConfigConfigMapRep(configMap *corev1.ConfigMap) *model.Packet {
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
