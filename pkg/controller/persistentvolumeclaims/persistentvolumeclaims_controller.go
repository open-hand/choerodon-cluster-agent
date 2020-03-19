package persistentvolumeclaims

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/golang/glog"

	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
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

var log = logf.Log.WithName("controller_persistentVolumeClaim")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new PersistentVolumeClaim Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcilePersistentVolumeClaim{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("persistentVolumeClaim-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource PersistentVolumeClaim
	err = c.Watch(&source.Kind{Type: &corev1.PersistentVolumeClaim{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcilePersistentVolumeClaim implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcilePersistentVolumeClaim{}

// ReconcilePersistentVolumeClaim reconciles a PersistentVolumeClaim object
type ReconcilePersistentVolumeClaim struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a PersistentVolumeClaim object and makes changes based on the state read
// and what is in the PersistentVolumeClaim.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcilePersistentVolumeClaim) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling PersistentVolumeClaim")

	// Fetch the PersistentVolumeClaim instance
	pvc := &corev1.PersistentVolumeClaim{}
	err := r.client.Get(context.TODO(), request.NamespacedName, pvc)

	responseChan := r.args.CrChan.ResponseChan
	if err != nil {
		if errors.IsNotFound(err) {
			responseChan <- newPersistentVolumeClaimDelRep(request.Name, request.Namespace)
			glog.Warningf("persistentVolumeClaim '%s' in work queue no longer exists", request.Name)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if pvc.Labels[model.AgentVersionLabel] != "" {
		responseChan <- newPersistentVolumeClaimRepoRep(pvc)
	}

	return reconcile.Result{}, nil
}

func newPersistentVolumeClaimDelRep(name string, namespace string) *model.Packet {

	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.PersistentVolumeClaim:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newPersistentVolumeClaimRepoRep(persistentVolumeClaim *corev1.PersistentVolumeClaim) *model.Packet {
	payload, err := json.Marshal(persistentVolumeClaim)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.PersistentVolumeClaim:%s", persistentVolumeClaim.Namespace, persistentVolumeClaim.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
