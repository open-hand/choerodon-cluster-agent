package daemonset

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	v1 "k8s.io/api/apps/v1"
	"strings"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_daemonset")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new DaemonSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileDaemonSet{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("daemonset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource DaemonSet
	err = c.Watch(&source.Kind{Type: &v1.DaemonSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner DaemonSet
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1.DaemonSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDaemonSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDaemonSet{}

// ReconcileDaemonSet reconciles a DaemonSet object
type ReconcileDaemonSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a DaemonSet object and makes changes based on the state read
// and what is in the DaemonSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDaemonSet) Reconcile(ctx context.Context,request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling DaemonSet")

	responseChan := r.args.CrChan.ResponseChan

	// Fetch the DaemonSet instance
	instance := &v1.DaemonSet{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			responseChan <- newDaemonSetDelRep(request.Name, request.Namespace)
			glog.Warningf("deamonset '%s' in work queue no longer exists", request.Name)
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
		responseChan <- newDaemonSetRep(instance)
	} else if instance.Labels[model.WorkloadLabel] != "" {
		responseChan <- newRepoDaemonSetRep(instance)
	}

	return reconcile.Result{}, nil
}

func newDaemonSetDelRep(name string, namespace string) *model.Packet {

	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.DaemonSet:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newDaemonSetRep(daemonSet *v1.DaemonSet) *model.Packet {
	payload, err := json.Marshal(daemonSet)
	release := daemonSet.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.DaemonSet:%s", daemonSet.Namespace, release, daemonSet.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func newRepoDaemonSetRep(daemonSet *v1.DaemonSet) *model.Packet {
	payload, err := json.Marshal(daemonSet)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.DaemonSet:%s", daemonSet.Namespace, daemonSet.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
