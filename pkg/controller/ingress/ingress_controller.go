package ingress

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	extensionsv1beta1 "k8s.io/api/extensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strings"
)

var log = logf.Log.WithName("controller_ingress")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Ingress Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileIngress{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("ingress-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Ingress
	err = c.Watch(&source.Kind{Type: &extensionsv1beta1.Ingress{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileIngress implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileIngress{}

// ReconcileIngress reconciles a Ingress object
type ReconcileIngress struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Ingress object and makes changes based on the state read
// and what is in the Ingress.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileIngress) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	if !r.args.Namespaces.Contain(request.Namespace) {
		return reconcile.Result{}, nil
	}

	responseChan := r.args.CrChan.ResponseChan

	// Fetch the Ingress instance
	instance := &extensionsv1beta1.Ingress{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			responseChan <- newIngressDelRep(request.Name, request.Namespace)
			glog.Warningf("ingress '%s' in work queue no longer exists", request.Name)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	if strings.Contains(instance.Labels["release"], "prometheus-operator") {
		return reconcile.Result{}, nil
	}

	if instance.Labels[model.NetworkLabel] != "" {
		glog.V(2).Info(instance.Labels[model.NetworkLabel], ":", instance)
		responseChan <- newIngressRep(instance)
	}

	return reconcile.Result{}, nil
}

func newIngressDelRep(name string, namespace string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.Ingress:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newIngressRep(ingress *extensionsv1beta1.Ingress) *model.Packet {
	payload, err := json.Marshal(ingress)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Ingress:%s", ingress.Namespace, ingress.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

// TODO ingress sync
//func (c *controller) resourceSync() {
//	namespaces := c.namespaces.GetAll()
//	for _, ns := range namespaces {
//		pods, err := c.lister.Ingresses(ns).List(labels.NewSelector())
//		if err != nil {
//			glog.Fatal("can not list resource, no rabc bind, exit !")
//		} else {
//			var serviceList []string
//			for _, pod := range pods {
//				if pod.Labels[model.ReleaseLabel] != "" {
//					serviceList = append(serviceList, pod.GetName())
//				}
//			}
//			resourceList := &kubernetes.ResourceList{
//				Resources:    serviceList,
//				ResourceType: "Ingress",
//			}
//			content, err := json.Marshal(resourceList)
//			if err != nil {
//				glog.Fatal("marshal ingress list error")
//			} else {
//				response := &model.Packet{
//					Key:     fmt.Sprintf("env:%s", ns),
//					Type:    model.ResourceSync,
//					Payload: string(content),
//				}
//				c.responseChan <- response
//			}
//		}
//	}
//}
