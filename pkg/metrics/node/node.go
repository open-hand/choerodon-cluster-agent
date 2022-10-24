package node

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/sets"
	client "k8s.io/client-go/kubernetes"
	"strings"
	"time"
)

const (
	LabelNodeRolePrefix = "node-role.kubernetes.io/"
	// NodeLabelRole specifies the role of a node
	RoleLabel = "kubernetes.io/role"
)

type NodeInfo struct {
	NodeName          string `json:"nodeName,omitempty"`
	Status            string `json:"status,omitempty"`
	Type              string `json:"role,omitempty"`
	CreateTime        string `json:"createTime,omitempty"`
	CpuCapacity       string `json:"cpuCapacity,omitempty"`
	CpuAllocatable    string `json:"cpuAllocatable,omitempty"`
	PodAllocatable    string `json:"podAllocatable,omitempty"`
	PodCapacity       string `json:"podCapacity,omitempty"`
	MemoryCapacity    string `json:"memoryCapacity,omitempty"`
	MemoryAllocatable string `json:"memoryAllocatable,omitempty"`
	MemoryRequest     string `json:"memoryRequest,omitempty"`
	MemoryLimit       string `json:"memoryLimit,omitempty"`
	CpuRequest        string `json:"cpuRequest,omitempty"`
	CpuLimit          string `json:"cpuLimit,omitempty"`
	PodCount          int    `json:"podCount,omitempty"`
}

type Node struct {
	Client client.Interface
	CrChan *channel.CRChan
}

func (no *Node) Run(stopCh <-chan struct{}) error {
	responseChan := no.CrChan.ResponseChan
	for {
		select {
		case <-stopCh:
			glog.Info("stop node metrics collector")
			return nil
		case <-time.Tick(time.Second * 30):
			glog.Infof("[Goroutine %d] node_sync", util.GetGID())
			nodes := make([]NodeInfo, 0)
			nodeList, err := no.Client.CoreV1().Nodes().List(context.TODO(), v1.ListOptions{})
			if err != nil {
				glog.Errorf("list node error :", err)
				continue
			}
			if len(nodeList.Items) == 0 {
				glog.Info("The count of listed node is 0.")
				continue
			}
			for _, node := range nodeList.Items {
				fieldSelector, err := fields.ParseSelector("spec.nodeName=" + node.Name + ",status.phase!=" + string(corev1.PodSucceeded) + ",status.phase!=" + string(corev1.PodFailed))
				if err != nil {
					glog.Errorf("parse field selector error: %v", err)
					continue
				}
				podList, err := no.Client.CoreV1().Pods("").List(context.TODO(), v1.ListOptions{
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
					role = strings.Join(roles, ",")
				}
				reqs, limit, err := getPodsTotalRequestsAndLimits(podList)
				if err != nil {
					glog.Errorf("failed to get pods total request and limits: %v", err)
					continue
				}
				CpuLimit := limit["cpu"]
				CpuRequest := reqs["cpu"]
				MemoryRequest := reqs["memory"]
				MemoryLimit := limit["memory"]
				nodeInfo := &NodeInfo{
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
				responseChan <- response
			}
		}
	}
}

func getPodsTotalRequestsAndLimits(podList *corev1.PodList) (reqs map[corev1.ResourceName]resource.Quantity, limits map[corev1.ResourceName]resource.Quantity, err error) {
	reqs, limits = map[corev1.ResourceName]resource.Quantity{}, map[corev1.ResourceName]resource.Quantity{}
	for _, pod := range podList.Items {
		podReqs, podLimits, err := PodRequestsAndLimits(&pod)
		if err != nil {
			return nil, nil, err
		}
		for podReqName, podReqValue := range podReqs {
			if value, ok := reqs[podReqName]; !ok {
				quantity, err := quantityCopy(&podReqValue)
				if err != nil {
					return nil, nil, err
				}
				reqs[podReqName] = *quantity
			} else {
				value.Add(podReqValue)
				reqs[podReqName] = value
			}
		}
		for podLimitName, podLimitValue := range podLimits {
			if value, ok := limits[podLimitName]; !ok {
				quantity, err := quantityCopy(&podLimitValue)
				if err != nil {
					return nil, nil, err
				}
				limits[podLimitName] = *quantity
			} else {
				value.Add(podLimitValue)
				limits[podLimitName] = value
			}
		}
	}
	return
}
func PodRequestsAndLimits(pod *corev1.Pod) (reqs, limits corev1.ResourceList, err error) {
	reqs, limits = corev1.ResourceList{}, corev1.ResourceList{}
	for _, container := range pod.Spec.Containers {
		err = addResourceList(reqs, container.Resources.Requests)
		if err != nil {
			return nil, nil, err
		}
		err = addResourceList(limits, container.Resources.Limits)
		if err != nil {
			return nil, nil, err
		}
	}
	// init containers define the minimum of any resource
	for _, container := range pod.Spec.InitContainers {
		err = maxResourceList(reqs, container.Resources.Requests)
		if err != nil {
			return nil, nil, err
		}
		err = maxResourceList(limits, container.Resources.Limits)
		if err != nil {
			return nil, nil, err
		}
	}
	return
}

// addResourceList adds the resources in newList to list
func addResourceList(list, new corev1.ResourceList) error {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			quantity, err := quantityCopy(&quantity)
			if err != nil {
				return err
			}
			list[name] = *quantity
		} else {
			value.Add(quantity)
			list[name] = value
		}
	}
	return nil
}

// maxResourceList sets list to the greater of list/newList for every resource
// either list
func maxResourceList(list, new corev1.ResourceList) error {
	for name, quantity := range new {
		if value, ok := list[name]; !ok {
			quantity, err := quantityCopy(&quantity)
			if err != nil {
				return err
			}
			list[name] = *quantity
			continue
		} else {
			if quantity.Cmp(value) > 0 {
				quantity, err := quantityCopy(&quantity)
				if err != nil {
					return err
				}
				list[name] = *quantity
			}
		}
	}
	return nil
}

func findNodeRoles(node *corev1.Node) []string {
	roles := sets.NewString()
	for k, v := range node.Labels {
		switch {
		case strings.HasPrefix(k, LabelNodeRolePrefix):
			if role := strings.TrimPrefix(k, LabelNodeRolePrefix); len(role) > 0 {
				roles.Insert(role)
			}

		case k == RoleLabel && v != "":
			roles.Insert(v)
		}
	}
	return roles.List()
}

func quantityCopy(q *resource.Quantity) (*resource.Quantity, error) {
	data, err := q.Marshal()
	if err != nil {
		return nil, err
	}
	quantity := &resource.Quantity{}
	err = quantity.Unmarshal(data)
	if err != nil {
		return nil, err
	}
	return quantity, nil
}
