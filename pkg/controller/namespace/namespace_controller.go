package namespace

import (
	"context"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/packet"

	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	"sigs.k8s.io/controller-runtime/pkg/source"
)

var log = logf.Log.WithName("controller_namespace")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Namespace Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	if model.RestrictedModel {
		return nil
	}
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileNamespace{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("namespace-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Namespace
	err = c.Watch(&source.Kind{Type: &corev1.Namespace{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}
	return nil
}

// blank assignment to verify that ReconcileNamespace implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileNamespace{}

// ReconcileNamespace reconciles a Namespace object
type ReconcileNamespace struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Namespace object and makes changes based on the state read
// and what is in the Namespace.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileNamespace) Reconcile(ctx context.Context,request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Namespace")

	// Fetch the Namespace instance
	namespaces := &corev1.NamespaceList{}
	opts := &client.ListOptions{}
	err := r.client.List(context.TODO(), namespaces, opts)
	if err != nil {
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	r.args.CrChan.ResponseChan <- packet.NamespacePacket(namespaces)

	return reconcile.Result{}, nil
}
