package c7nhelmrelease

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	choerodonv1alpha1 "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	modelhelm "github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions"
	apiextensions_v1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	runtimeutil "k8s.io/apimachinery/pkg/util/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller"
	"sigs.k8s.io/controller-runtime/pkg/handler"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	"sigs.k8s.io/controller-runtime/pkg/reconcile"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"sigs.k8s.io/controller-runtime/pkg/source"
	"strconv"
)

var log = logf.Log.WithName("controller_c7nhelmrelease")

// 会有commandId和appServiceId的资源类型
var labeledResourceKinds = []string{"ReplicationController", "ReplicaSet", "Deployment", "Job", "DaemonSet", "StatefulSet"}

/**
* USER ACTION REQUIRED: This is a scaffold file intended for the user to modify with their own Controller
* business logic.  Delete these comments after modifying this file.*
 */

// Add creates a new C7NHelmRelease Controller and adds it to the Manager. The Manager will set fields on the Controller
// and Start it when the Manager is Started.
func Add(mgr manager.Manager, args *controllerutil.Args) error {
	return add(mgr, newReconciler(mgr, args))
}

// newReconciler returns a new reconcile.Reconciler
func newReconciler(mgr manager.Manager, args *controllerutil.Args) reconcile.Reconciler {
	return &ReconcileC7NHelmRelease{client: mgr.GetClient(), scheme: mgr.GetScheme(), args: args}
}

// add adds a new Controller to mgr with r as the reconcile.Reconciler
func add(mgr manager.Manager, r reconcile.Reconciler) error {
	// Create a new controller
	c, err := controller.New("c7nhelmrelease-controller", mgr, controller.Options{Reconciler: r})
	if err != nil {
		return err
	}

	// Watch for changes to primary resource C7NHelmRelease
	err = c.Watch(&source.Kind{Type: &choerodonv1alpha1.C7NHelmRelease{}}, &handler.EnqueueRequestForObject{})
	if err != nil {
		return err
	}

	return nil
}

// blank assignment to verify that ReconcileC7NHelmRelease implements reconcile.Reconciler
var _ reconcile.Reconciler = &ReconcileC7NHelmRelease{}

// ReconcileC7NHelmRelease reconciles a C7NHelmRelease object
type ReconcileC7NHelmRelease struct {
	// This client, initialized using mgr.Client() above, is a split client
	// that reads objects from the cache and writes to the apiserver
	client client.Client
	scheme *runtime.Scheme
	args   *controllerutil.Args
}

// Reconcile reads that state of the cluster for a C7NHelmRelease object and makes changes based on the state read
// and what is in the C7NHelmRelease.Spec
// TODO(user): Modify this Reconcile function to implement your Controller logic.  This example creates
// a Pod as an example
// Note:
// The Controller will requeue the Request to be processed again if the returned error is non-nil or
// ResourceList.Requeue is true, otherwise upon completion it will remove the work from the queue.
func (r *ReconcileC7NHelmRelease) Reconcile(request reconcile.Request) (reconcile.Result, error) {
	namespace := request.Namespace
	//对应实例的名字 如：helm-lll-1f3b8
	name := request.Name
	reqLogger := log.WithValues("Request.Namespace", request.Namespace, "Request.Name", request.Name)
	reqLogger.Info("Reconciling C7NHelmRelease")

	commandChan := r.args.CrChan.CommandChan

	result := reconcile.Result{}

	// Fetch the C7NHelmRelease instance
	instance := &choerodonv1alpha1.C7NHelmRelease{}
	// 获得集群中指定名称的C7NHelmRelease
	err := r.client.Get(context.TODO(), request.NamespacedName, instance)

	if err != nil {
		if errors.IsNotFound(err) {
			// 判断C7NHelmRelease资源是否存在，不存在表示实例删除操作
			if !r.checkCrdDeleted(instance) {
				runtimeutil.HandleError(fmt.Errorf("C7NHelmReleases '%s' in work queue no longer exists", name))
				if cmd := deleteHelmReleaseCmd(namespace, name); cmd != nil {
					glog.Infof("release %s delete", name)
					commandChan <- cmd
				}
			} else {
				glog.Warningf("C7NHelmReleases crd not exist")
			}
			return result, nil
		}
		// Error reading the object - requeue the request.
		return result, err
	}

	// 如果该资源在choerodon命名空间下，那么判断该资源的所属集群id与当前agent的clusterId是否相同，
	// 相同表示是该agent的资源
	// 不同则表示不是该agent资源，不进行处理
	if namespace == kube.AgentNamespace {
		if instance.Labels[model.C7NHelmReleaseClusterLabel] != strconv.Itoa(int(kube.ClusterId)) {
			return result, nil
		}
	}

	if instance.Annotations == nil || instance.Annotations[model.CommitLabel] == "" {
		return result, fmt.Errorf("c7nhelmrelease has no commit annotations")
	}

	operate, ok := instance.Annotations[model.C7NHelmReleaseOperateAnnotation]
	if ok {
		switch operate {
		case model.INSTALL:
			// 安装操作
			glog.Infof("release %s not found", instance.Name)
			if cmd := installHelmReleaseCmd(instance); cmd != nil {
				glog.Infof("release %s start to install", instance.Name)
				commandChan <- cmd
			}
			break
		case model.UPGRADE:
			// 升级操作
			if cmd := updateHelmReleaseCmd(instance); cmd != nil {
				glog.Infof("release %s upgrade", instance.Name)
				commandChan <- cmd
			}
		}
		annotations := instance.Annotations
		delete(annotations, model.C7NHelmReleaseOperateAnnotation)
		instance.SetAnnotations(annotations)
		err = r.client.Update(context.TODO(), instance)
		if err != nil {
			glog.Info(err)
			return reconcile.Result{}, err
		}
		return reconcile.Result{}, nil
	}
	return reconcile.Result{}, nil
}

func installHelmReleaseCmd(instance *choerodonv1alpha1.C7NHelmRelease) *model.Packet {
	req := &modelhelm.InstallReleaseRequest{
		RepoURL:          instance.Spec.RepoURL,
		ChartName:        instance.Spec.ChartName,
		ChartVersion:     instance.Spec.ChartVersion,
		Values:           instance.Spec.Values,
		ReleaseName:      instance.Name,
		Command:          instance.Spec.CommandId,
		Commit:           instance.Annotations[model.CommitLabel],
		Namespace:        instance.Namespace,
		AppServiceId:     instance.Spec.AppServiceId,
		ImagePullSecrets: instance.Spec.ImagePullSecrets,
		FailedCount:      0,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", instance.Namespace, instance.Name),
		Type:    model.HelmInstallJobInfo,
		Payload: string(reqBytes),
	}
}

// update command
func updateHelmReleaseCmd(instance *choerodonv1alpha1.C7NHelmRelease) *model.Packet {
	req := &modelhelm.UpgradeReleaseRequest{
		RepoURL:          instance.Spec.RepoURL,
		ChartName:        instance.Spec.ChartName,
		ChartVersion:     instance.Spec.ChartVersion,
		Values:           instance.Spec.Values,
		ReleaseName:      instance.Name,
		Command:          instance.Spec.CommandId,
		Commit:           instance.Annotations[model.CommitLabel],
		Namespace:        instance.Namespace,
		AppServiceId:     instance.Spec.AppServiceId,
		ImagePullSecrets: instance.Spec.ImagePullSecrets,
	}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", instance.Namespace, instance.Name),
		Type:    model.HelmUpgradeJobInfo,
		Payload: string(reqBytes),
	}
}

//
func newReleaseSyncFailRep(instance *choerodonv1alpha1.C7NHelmRelease, msg string) *model.Packet {
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.commit:%s", instance.Namespace, instance.Name, instance.Annotations[model.CommitLabel]),
		Type:    model.HelmReleaseSyncedFailed,
		Payload: msg,
	}
}

func UpgradeInstanceStatusCmd(instance *choerodonv1alpha1.C7NHelmRelease, payload string) *model.Packet {
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s.commit:%s", instance.Namespace, instance.Name, instance.Annotations[model.CommitLabel]),
		Type:    model.HelmReleaseUpgradeResourceInfo,
		Payload: payload,
	}
}

// delete helm release
func deleteHelmReleaseCmd(namespace, name string) *model.Packet {
	req := &modelhelm.DeleteReleaseRequest{ReleaseName: name, Namespace: namespace}
	reqBytes, err := json.Marshal(req)
	if err != nil {
		glog.Error(err)
		return nil
	}
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s.release:%s", namespace, name),
		Type:    model.HelmReleaseDelete,
		Payload: string(reqBytes),
	}
}

func (r *ReconcileC7NHelmRelease) checkCrdDeleted(instance *choerodonv1alpha1.C7NHelmRelease) bool {
	found := &apiextensions_v1.CustomResourceDefinition{}
	err := r.client.Get(context.TODO(), types.NamespacedName{Name: "c7nhelmreleases.choerodon.io", Namespace: ""}, found)

	if err != nil && errors.IsNotFound(err) {
		return true
	} else if err != nil {
		glog.Error(err)
	}
	return false
}

func newC7NHelmCRDForCr(cr *choerodonv1alpha1.C7NHelmRelease) *apiextensions.CustomResourceDefinition {
	return &apiextensions.CustomResourceDefinition{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "c7nhelmreleases",
			Namespace: cr.Namespace,
		},
	}
}
