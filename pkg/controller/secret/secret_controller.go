package secret

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
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

var log = logf.Log.WithName("controller_secret")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Secret Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileSecret{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("secret-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Secret
	err = c.Watch(&source.Kind{Type: &corev1.Secret{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileSecret implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileSecret{}

// ReconcileSecret reconciles a Secret object
type ReconcileSecret struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Secret object and makes changes based on the state read
// and what is in the Secret.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileSecret) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Secret")

	// Fetch the Secret instance
	instance := &corev1.Secret{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	responseChan := r.args.CrChan.ResponseChan
	if err != nil {
		if errors.IsNotFound(err) {

			responseChan <- newsecretDelRep(request.Name, request.Namespace)
			glog.Warningf("secret '%s' in work queue no longer exists", instance)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if instance.Labels[model.ReleaseLabel] != "" {
		glog.V(2).Info(instance.Labels[model.ReleaseLabel], ":", instance)
		responseChan <- newsecretRep(instance)
	} else if string(instance.Type) == "kubernetes.io/tls" && instance.Annotations != nil && instance.Annotations[model.CommitLabel] != "" {
		if newCertIssuedRep(instance) != nil {
			responseChan <- newCertIssuedRep(instance)
		}
	} else if instance.Labels[model.AgentVersionLabel] != "" {
		responseChan <- newSecretRepoRep(instance)
	}

	return reconcile.Result{}, nil
}

func newsecretDelRep(name string, namespace string) *model.Packet {

	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.Secret:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newsecretRep(secret *corev1.Secret) *model.Packet {
	payload, err := json.Marshal(secret)
	release := secret.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Secret:%s", secret.Namespace, release, secret.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func newCertIssuedRep(secret *corev1.Secret) *model.Packet {
	payload, err := json.Marshal(secret)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Cert:%s.commit:%s", secret.Namespace, secret.Name, secret.Annotations[model.CommitLabel]),
		Type:    model.Cert_Issued,
		Payload: string(payload),
	}
}

func newSecretRepoRep(secret *corev1.Secret) *model.Packet {
	payload, err := json.Marshal(secret)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Secret:%s", secret.Namespace, secret.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
