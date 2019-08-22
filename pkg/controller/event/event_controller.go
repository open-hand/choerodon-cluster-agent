package event

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/golang/glog"
	"strings"

	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	batchv1 "k8s.io/api/batch/v1"
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

var log = logf.Log.WithName("controller_event")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Event Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileEvent{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("event-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Event
	err = c.Watch(&source.Kind{Type: &corev1.Event{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Event
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &corev1.Event{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileEvent implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileEvent{}

// ReconcileEvent reconciles a Event object
type ReconcileEvent struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Event object and makes changes based on the state read
// and what is in the Event.Spec
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileEvent) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	if !r.args.Namespaces.Contain(request.Namespace) {
		return reconcile.Result{}, nil
	}

	responseChan := r.args.CrChan.ResponseChan

	// Fetch the Event instance
	instance := &corev1.Event{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			// Request object not found, could have been deleted after reconcile request.
			// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
			// Return and don't requeue
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	switch instance.InvolvedObject.Kind {
	case "Job":
		job := &batchv1.Job{}
		err := r.client.Get(context.TODO(),
			client.ObjectKey{Namespace: instance.InvolvedObject.Namespace, Name: instance.InvolvedObject.Name},
			job)
		if err != nil {
			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}
		if job.Labels[model.TestLabel] == "" {
			responseChan <- newEventRep(instance)
		}
	case "Pod":
		pod := &corev1.Pod{}
		err := r.client.Get(context.TODO(),
			client.ObjectKey{Namespace: instance.InvolvedObject.Namespace, Name: instance.InvolvedObject.Name},
			pod)
		if err != nil {
			if errors.IsNotFound(err) {
				// Request object not found, could have been deleted after reconcile request.
				// Owned objects are automatically garbage collected. For additional cleanup logic use finalizers.
				// Return and don't requeue
				return reconcile.Result{}, nil
			}
			// Error reading the object - requeue the request.
			return reconcile.Result{}, err
		}
		if pod.Labels[model.ReleaseLabel] != "" && pod.Labels[model.TestLabel] == "" {
			//实例pod产生的事件
			responseChan <- newInstanceEventRep(instance, pod.Labels[model.ReleaseLabel])
			return reconcile.Result{}, nil

		} else if pod.Labels[model.TestLabel] == r.args.PlatformCode {
			//测试pod事件
			responseChan <- newTestPodEventRep(instance, pod.Labels[model.ReleaseLabel], pod.Labels[model.TestLabel])
		} else {
			responseChan <- newEventRep(instance)
		}
	case "Certificate":
		if strings.Contains(instance.Reason, "Err") {
			if len(instance.Message) > 43 {
				glog.Warningf("Certificate err event reason not contain commit")
				responseChan <- newCertFailed(instance)
			}
		}
	}

	return reconcile.Result{}, nil
}

func newEventRep(event *corev1.Event) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Event:%s", event.Namespace, event.Name),
		Type:    model.HelmJobEvent,
		Payload: string(payload),
	}
}

func newInstanceEventRep(event *corev1.Event, release string) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Event:%s", event.Namespace, release, event.Name),
		Type:    model.HelmPodEvent,
		Payload: string(payload),
	}
}

func newTestPodEventRep(event *corev1.Event, release string, label string) *model.Packet {
	payload, err := json.Marshal(event)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.label:%s", event.Namespace, release, label),
		Type:    model.TestPodEvent,
		Payload: string(payload),
	}
}

func newCertFailed(event *corev1.Event) *model.Packet {
	commit := event.Message[1:41]
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.Cert:%s.commit:%s", event.Namespace, event.InvolvedObject.Name, commit),
		Type:    model.Cert_Faild,
		Payload: event.Message[42:],
	}
}
