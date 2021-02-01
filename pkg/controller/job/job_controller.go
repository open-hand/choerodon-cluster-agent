package job

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	v1 "k8s.io/api/batch/v1"
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

var log = logf.Log.WithName("controller_job")

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new Job Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileJob{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("job-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource Job
	err = c.Watch(&source.Kind{Type: &v1.Job{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	// TODO(user): Modify this to be the types you create that are owned by the primary resource
	// Watch for changes to secondary resource Pods and requeue the owner Job
	err = c.Watch(&source.Kind{Type: &corev1.Pod{}}, &handler.EnqueueRequestForOwner{
		IsController: true,
		OwnerType:    &v1.Job{},
	})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileJob implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileJob{}

// ReconcileJob reconciles a Job object
type ReconcileJob struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a Job object and makes changes based on the state read
// and what is in the Job.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// Result.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileJob) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling Job")

	responseChan := r.args.CrChan.ResponseChan
	namespace := request.Namespace

	// Fetch the Job instance
	instance := &v1.Job{}
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)
	if err != nil {
		if errors.IsNotFound(err) {
			responseChan <- newJobDelRep(request.Name, request.Namespace)
			glog.Warningf("job '%s' in work queue no longer exists", request.Name)
			return reconcile.Result{}, nil
		}
		// Error reading the object - requeue the request.
		return reconcile.Result{}, err
	}

	kubeClient := r.args.KubeClient

	if instance.Labels[model.ReleaseLabel] != "" && instance.Labels[model.TestLabel] == "" {
		glog.V(2).Info(instance.Labels[model.ReleaseLabel], ":", instance)
		responseChan <- newJobRep(instance)
		if finish, _ := IsJobFinished(instance); finish {
			jobLogs, _, err := kubeClient.LogsForJob(request.Namespace, instance.Name, model.ReleaseLabel)
			if err != nil {
				glog.Error("get job log error ", err)
			} else if strings.TrimSpace(jobLogs) != "" {
				lobLogLength := len(jobLogs)
				if lobLogLength > 20480 {
					jobLogs = jobLogs[lobLogLength-20480:lobLogLength]
				}
				responseChan <- newJobLogRep(instance.Name, instance.Labels[model.ReleaseLabel], jobLogs, request.Namespace)
			}
			err = kubeClient.DeleteJob(namespace, instance.Name)
			if err != nil {
				glog.Error("delete job error", err)
			}
		}

	} else if instance.Labels[model.TestLabel] != "" && instance.Labels[model.TestLabel] == r.args.PlatformCode {
		//监听
		if finsish, succeed := IsJobFinished(instance); finsish {
			jobLogs, jobstatus, err := kubeClient.LogsForJob(namespace, instance.Name, model.TestLabel)

			if succeed == false && jobstatus == "success" {
				succeed = true
			}

			if err != nil {
				glog.Error("get job log error ", err)
			} else if strings.TrimSpace(jobLogs) != "" {
				responseChan <- newTestJobLogRep(instance.Labels[model.TestLabel], instance.Labels[model.ReleaseLabel], jobLogs, namespace, succeed)
			}
			_, err = r.args.HelmClient.DeleteRelease(&helm.DeleteReleaseRequest{ReleaseName: instance.Labels[model.ReleaseLabel], Namespace: helm.TestNamespace})
			if err != nil {
				glog.Errorf("error delete release %s: ", instance.Labels[model.ReleaseLabel], err)
			}
		}
	}

	return reconcile.Result{}, nil
}

func newJobDelRep(name string, namespace string) *model.Packet {

	return &model.Packet{
		Key:  fmt.Sprintf("env:%s.Job:%s", namespace, name),
		Type: model.ResourceDelete,
	}
}

func newJobLogRep(name string, release string, jobLogs string, namespace string) *model.Packet {
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Job:%s", namespace, release, name),
		Type:    model.HelmJobLog,
		Payload: jobLogs,
	}
}

func newTestJobLogRep(label string, release string, jobLogs string, namespace string, succeed bool) *model.Packet {
	rsp := &helm.TestJobFinished{
		Succeed: succeed,
		Log:     jobLogs,
	}
	rspBytes, err := json.Marshal(rsp)
	if err != nil {
		glog.Errorf("marshal test job rsp error: %v", err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.label:%s", namespace, release, label),
		Type:    model.TestJobLog,
		Payload: string(rspBytes),
	}
}

func newJobRep(job *v1.Job) *model.Packet {
	payload, err := json.Marshal(job)
	release := job.Labels[model.ReleaseLabel]
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.Job:%s", job.Namespace, release, job.Name),
		Type:    model.ResourceUpdate,
		Payload: string(payload),
	}
}

func IsJobFinished(j *v1.Job) (bool, bool) {
	for _, c := range j.Status.Conditions {
		if c.Status == "True" {
			if c.Type == v1.JobComplete {

				return true, true
			} else if c.Type == v1.JobFailed {
				return true, false
			}
		}
	}
	return false, false
}
