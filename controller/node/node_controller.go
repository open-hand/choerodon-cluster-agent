package node

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/sets"
	clientset "k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/cache"
	"strings"
	"time"
)

var (
	keyFunc             = cache.DeletionHandlingMetaNamespaceKeyFunc
	LabelNodeRolePrefix = "node-role.kubernetes.io/"

	// NodeLabelRole specifies the role of a node
	NodeLabelRole = "kubernetes.io/role"
)

type controller struct {
	responseChan  chan<- *model.Packet
	namespaces    *manager.Namespaces
	platformCode  string
	kubeClientset clientset.Interface
}

func NewNodeController(kubeClientset clientset.Interface, responseChan chan<- *model.Packet, namespaces *manager.Namespaces, platformCode string) *controller {

	c := &controller{
		kubeClientset: kubeClientset,
		responseChan:  responseChan,
		namespaces:    namespaces,
		platformCode:  platformCode,
	}
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {

	syncTimer := time.NewTimer(30 * time.Second)
	for {
		select {
		case <-stopCh:
			glog.Info("stop node controller")
			return
		case <-syncTimer.C:
			nodes := []kubernetes.NodeInfo{}
			nodelist, err := c.kubeClientset.CoreV1().Nodes().List(v1.ListOptions{})
			if err != nil {
				glog.Errorf("list node error :", err)
			}
			for _, node := range nodelist.Items {
				fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name + ",status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
				if err != nil {
					glog.Errorf("parse field selector error: %v", err)
					continue
				}
				podList, err := c.kubeClientset.CoreV1().Pods("").List(v1.ListOptions{
					FieldSelector: fieldSelector.String(),
				})
				if err != nil {
					glog.Errorf("list node pod error: %v", err)
					continue
				}
				roles := findNodeRoles(&node)
				var role string
				if len(roles) == 0 {
					role = "none"
				} else {
					role = strings.Join(roles, "")
				}
				reqs, limit := getPodsTotalRequestsAndLimits(podList)
				CpuLimit := limit["cpu"]
				CpuRequest := reqs["cpu"]
				MemoryRequest := reqs["memory"]
				MemoryLimit := limit["memory"]
				nodeInfo := &kubernetes.NodeInfo{
					NodeName:          node.Name,
					CreateTime:        node.CreationTimestamp.String(),
					CpuAllocatable:    node.Status.Allocatable.Cpu().String(),
					CpuCapacity:       node.Status.Capacity.Cpu().String(),
					CpuLimit:          CpuLimit.String(),
					CpuRequest:        CpuRequest.String(),
					MemoryCapacity:    node.Status.Capacity.Memory().String(),
					MemoryAllocatable: node.Status.Allocatable.Memory().String(),
					PodAllocatable:    node.Status.Allocatable.Pods().String(),
					PodCapacity:       node.Status.Capacity.Pods().String(),
					MemoryRequest:     MemoryRequest.String(),
					MemoryLimit:       MemoryLimit.String(),
					PodCount:          len(podList.Items),
					Type:              role,
				}
				for _, condition := range node.Status.Conditions {
					if string(condition.Status) == "True" {
						nodeInfo.Status = string(condition.Type)
					}
				}
				if nodeInfo.Status == "" {
					nodeInfo.Status = "Unknown"
				}
				nodes = append(nodes, *nodeInfo)

			}
			content, err := json.Marshal(nodes)
			if err != nil {
				glog.Fatal("marshal pod list error")
			} else {
				response := &model.Packet{
					Key:     fmt.Sprintf("inter:inter"),
					Type:    model.NodeSync,
					Payload: string(content),
				}
				c.responseChan <- response
			}
			syncTimer.Reset(30 * time.Second)
		}
	}

	// Launch two workers to process Foo resources

}

func getPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity) {
	reqs, limits = map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, pod := range podList.Items {
		podReqs, podLimits := PodRequestsAndLimits(&pod)
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				reqs[podReqName] = *podReqValue.Copy()
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				limits[podLimitName] = *podLimitValue.Copy()
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}
func PodRequestsAndLimits(pod *corev1.Pod) (reqs, limits corev1.ResourceList) {
	reqs, limits = corev1.ResourceList{}, corev1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		addResourceList(reqs, container.Resources.Requests)
		addResourceList(limits, container.Resources.Limits)
	}
	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		maxResourceList(reqs, container.Resources.Requests)
		maxResourceList(limits, container.Resources.Limits)
	}
	return
}

// addResourceList adds the resources in newList to list
func addResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = *quantity.Copy()
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, new corev1.ResourceList) {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			list[name] = *quantity.Copy()
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				list[name] = *quantity.Copy()
			}
		}
	}
}

func findNodeRoles(node *corev1.Node) []string {
	roles := sets.NewString()
	for k, v := range node.Labels {
		switch {
		case strings.HasPrefix(k, LabelNodeRolePrefix):
			if role := strings.TrimPrefix(k, LabelNodeRolePrefix); len(role) > 0 {
				roles.Insert(role)
			}

		case k == NodeLabelRole && v != "":
			roles.Insert(v)
		}
	}
	return roles.List()
}
