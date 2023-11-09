package sync

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	agentnamespace "github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/metrics"
	"github.com/choerodon/choerodon-cluster-agent/pkg/metrics/node"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/packet"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
)

type Context struct {
	Namespaces *agentnamespace.Namespaces
	KubeClient kube.Client
	HelmClient helm.Client
	CrChan     *channel.CRChan
	StopCh     chan struct{}
	stopCh     chan struct{}
}

var syncFuncs []func(ctx *Context) error

var initialized = false

func syncStatefulSet(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		instances, err := ctx.KubeClient.GetKubeClient().AppsV1().StatefulSets(ns).List(context.TODO(),metav1.ListOptions{})
		if err != nil {
			glog.Warning("StatefulSets v1 not support on your cluster ", err)
		} else {
			podList := make([]string, 0)
			for _, statefulset := range instances.Items {
				if statefulset.Labels[model.ReleaseLabel] != "" {
					podList = append(podList, statefulset.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    podList,
				ResourceType: "StatefulSet",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal pod list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncReplicaSet(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		rsList, err := ctx.KubeClient.GetKubeClient().AppsV1().ReplicaSets(ns).List(context.TODO(),metav1.ListOptions{})
		if err != nil {
			glog.Fatal(err)
		} else {
			resourceSyncList := make([]string, 0)
			for _, resource := range rsList.Items {
				if resource.Labels[model.ReleaseLabel] != "" {
					resourceSyncList = append(resourceSyncList, resource.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    resourceSyncList,
				ResourceType: "ReplicaSet",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal ReplicaSet list error:", err)
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncService(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {
		instances, err := ctx.KubeClient.GetKubeClient().CoreV1().Services(ns).List(context.TODO(),metav1.ListOptions{})
		if err != nil {
			glog.Fatal(err)
		} else {
			serviceList := make([]string, 0)
			for _, instance := range instances.Items {
				if instance.Labels[model.ReleaseLabel] != "" {
					serviceList = append(serviceList, instance.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    serviceList,
				ResourceType: "Service",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal service list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

// 将集群中的空间发给devOps防止冲突
func syncNamespace(ctx *Context) error {
	if model.RestrictedModel {
		return nil
	}
	namespaces, err := ctx.KubeClient.GetKubeClient().CoreV1().Namespaces().List(context.TODO(),metav1.ListOptions{})
	if err != nil {
		return err
	}
	ctx.CrChan.ResponseChan <- packet.NamespacePacket(namespaces)
	return nil
}

func syncPod(ctx *Context) error {
	namespaces := ctx.Namespaces.GetAll()
	for _, ns := range namespaces {

		pods, err := ctx.KubeClient.GetKubeClient().CoreV1().Pods(ns).List(context.TODO(),metav1.ListOptions{})
		if err != nil {
			glog.Fatal("can not list resource, no rabc bind, exit !")
		} else {
			podList := make([]string, 0)
			for _, pod := range pods.Items {
				if pod.Labels[model.ReleaseLabel] != "" {
					podList = append(podList, pod.GetName())
				}
			}
			resourceList := &ResourceList{
				Resources:    podList,
				ResourceType: "Pod",
			}
			content, err := json.Marshal(resourceList)
			if err != nil {
				glog.Fatal("marshal pod list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("env:%s", ns),
					Type:    model.ResourceSync,
					Payload: string(content),
				}
				ctx.CrChan.ResponseChan <- response
			}
		}
	}
	return nil
}

func syncStatus(ctx *Context) error {
	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	ticker := time.NewTicker(1 * time.Minute)
	for {
		select {
		case <-ctx.stopCh:
			glog.Info("sync loop stopping")
			return nil
		case <-ticker.C:
			for _, ns := range ctx.Namespaces.GetAll() {
				ctx.CrChan.ResponseChan <- newSyncRep(ns)
			}
		}
	}
}

func newSyncRep(ns string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s", ns),
		Type: model.ResourceStatusSyncEvent,
	}
}

func syncMetrics(ctx *Context) error {
	if model.RestrictedModel {
		return nil
	}
	m := &node.Node{
		Client: ctx.KubeClient.GetKubeClient(),
		CrChan: ctx.CrChan,
	}
	if !initialized {
		metrics.Register(m)
		initialized = true
	}
	metrics.Run(ctx.stopCh)
	return nil
}

func init() {
	syncFuncs = append(syncFuncs, syncStatefulSet)
	syncFuncs = append(syncFuncs, syncReplicaSet)
	syncFuncs = append(syncFuncs, syncService)
	syncFuncs = append(syncFuncs, syncPod)
	syncFuncs = append(syncFuncs, syncNamespace)
	syncFuncs = append(syncFuncs, syncStatus)
	syncFuncs = append(syncFuncs, syncMetrics)
}

func Run(ctx *Context) {
	for _, fn := range syncFuncs {
		p := fn
		go func() {
			if err := p(ctx); err != nil {
				glog.Warningf("sync %v failed", fn)
			}
		}()
	}
	go func() {
		for {
			select {
			case <-ctx.StopCh:
				close(ctx.stopCh)
				return
			}
		}
	}()
}

func (ctx *Context) ReSync() {
	// 有这种情况。节点同步逻辑正在进行中，这时close(ctx.stopCh)，不会终止节点同步协程。
	// 接着下面重新make，会导致节点同步逻辑执行完成后误认为没有接收到通道关闭信号，从而这个协程保留了下来。
	// 随着agent重连次数增多，节点同步的协程会越来越多
	// 所以睡眠等待10秒后再开始往下执行
	time.Sleep(10 * time.Second)
	if ctx.stopCh != nil {
		close(ctx.stopCh)
	}
	ctx.stopCh = make(chan struct{}, 1)
	Run(ctx)
}
