package deployment

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	v1 "k8s.io/api/apps/v1"

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

var log = logf.Log.WithName("controller_deployment")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Deployment Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileDeployment{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("deployment-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Deployment
	err = c.Watch(&source.Kind{Type: &v1.Deployment{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Deployment
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1.Deployment{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileDeployment implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileDeployment{}

// ReconcileDeployment reconciles a Deployment object
type ReconcileDeployment struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Deployment object and makes changes based on the state read
// and what is in the Deployment.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileDeployment) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Deployment")

	if !r.args.Namespaces.Contain(request.Namespace) {
		return reconcile.Result{}, nil
	}

	responseChan := r.args.CrChan.ResponseChan

	// Fetch the Deployment instance
	instance := &v1.Deployment{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			responseChan <- newDeploymentDelRep(request.Name, request.Namespace)
			glog.Warningf("deployment '%s' in work queue no longer exists", instance)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(instance.Labels[model.ReleaseLabel], ":", instance)
		responseChan <- newDeploymentRep(instance)
	}

	return reconcile.Result{}, nil
}

func newDeploymentDelRep(name string, namespace string) *model.Packet {

	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.Deployment:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newDeploymentRep(deployment *v1.Deployment) *model.Packet {
	payload, err := json.Marshal(deployment)
	release := deployment.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Deployment:%s", deployment.Namespace, release, deployment.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
