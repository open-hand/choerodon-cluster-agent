package statefulset

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/kubernetes"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	v1 "k8s.io/api/apps/v1"
	"k8s.io/api/apps/v1beta1"
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
	"strings"
)

var log = logf.Log.WithName("controller_statefulset")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new StatefulSet Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileStatefulSet{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("statefulset-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource StatefulSet
	err = c.Watch(&source.Kind{Type: &v1.StatefulSet{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner StatefulSet
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1beta1.StatefulSet{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileStatefulSet implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileStatefulSet{}

// ReconcileStatefulSet reconciles a StatefulSet object
type ReconcileStatefulSet struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a StatefulSet object and makes changes based on the state read
// and what is in the StatefulSet.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileStatefulSet) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling StatefulSet")

	commandChan := r.args.CrChan.CommandChan
	// Fetch the StatefulSet instance
	instance := &v1.StatefulSet{}
	responseChan := r.args.CrChan.ResponseChan
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			if strings.Contains(request.Name, "c7n-prometheus") {
				switch request.Name {
				// 如果是prometheus的prometheus，则删除其创建的PVC
				case "prometheus-c7n-prometheus-prometheus":
					pvcInfo := kubernetes.DeletePvcInfo{Namespace: request.Namespace, Labels: map[string]string{"app": "prometheus"}}
					commandChan <- deletePvcCmd(pvcInfo)
					// 如果是prometheus的alertmanager，则删除其创建的PVC
				case "alertmanager-c7n-prometheus-alertmanager":
					pvcInfo := kubernetes.DeletePvcInfo{Namespace: request.Namespace, Labels: map[string]string{"app": "alertmanager"}}
					commandChan <- deletePvcCmd(pvcInfo)
				}
			}
			responseChan <- newPodDelRep(request.Name, request.Namespace)
			glog.Warningf("pod '%s' in work queue no longer exists", request.Name)
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
		responseChan <- newPodRep(instance)
	} else if instance.Labels[model.WorkloadLabel] != "" {
		responseChan <- newRepoStatefulSetRep(instance)
	}

	return reconcile.Result{}, nil
}

func newPodDelRep(name string, namespace string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.StatefulSet:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newPodRep(instance *v1.StatefulSet) *model.Packet {
	payload, err := json.Marshal(instance)
	release := instance.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.StatefulSet:%s", instance.Namespace, release, instance.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

// delete pvc command
func deletePvcCmd(pvcInfo kubernetes.DeletePvcInfo) *model.Packet {
	reqBytes, err := json.Marshal(pvcInfo)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.PersistentVolumeClaimLabels:%s", pvcInfo.Namespace, pvcInfo.Labels),
		Type:    model.DeletePersistentVolumeClaimByLabels,
		Payload: string(reqBytes),
	}
}

func newRepoStatefulSetRep(statefulSet *v1.StatefulSet) *model.Packet {
	payload, err := json.Marshal(statefulSet)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.StatefulSet:%s", statefulSet.Namespace, statefulSet.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}
