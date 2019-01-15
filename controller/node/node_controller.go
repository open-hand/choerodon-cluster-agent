package node

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/labels"
	v1_informer "k8s.io/client-go/informers/core/v1"
	v1_lister "k8s.io/client-go/listers/core/v1"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/util/workqueue"
	"time"
)

var (
	keyFunc = cache.DeletionHandlingMetaNamespaceKeyFunc
)

type controller struct {
	queue workqueue.RateLimitingInterface
	// workerLoopPeriod is the time between worker runs. The workers process the queue of pod and pod changes.
	workerLoopPeriod time.Duration
	lister           v1_lister.NodeLister
	responseChan     chan<- *model.Packet
	synced       cache.InformerSynced
	namespaces       *manager.Namespaces
	platformCode     string
}

func NewNodeController(nodeInformer v1_informer.NodeInformer, responseChan chan<- *model.Packet, namespaces *manager.Namespaces, platformCode string) *controller {

	c := &controller{
		queue:            workqueue.NewNamedRateLimitingQueue(workqueue.DefaultControllerRateLimiter(), "pod"),
		workerLoopPeriod: time.Second,
		lister:           nodeInformer.Lister(),
		responseChan:     responseChan,
		namespaces:        namespaces,
		platformCode:    platformCode,
	}
	return c
}

func (c *controller) Run(workers int, stopCh <-chan struct{}) {

	syncTimer := time.NewTimer(30 * time.Second)
	for{
		select {
		case <-stopCh:
			glog.Info("stop node controller")
			return
		case <- syncTimer.C:
			nodes := []kubernetes.NodeInfo{}
			nodelist,err := c.lister.List(labels.NewSelector())
			if err != nil {
				glog.Errorf("list node error :", err)
			}
			for _, node := range  nodelist {

				nodeInfo := &kubernetes.NodeInfo{
					NodeName: node.Name,
					CreateTime:    node.CreationTimestamp.String(),
					CpuRequest:  node.Status.Allocatable.Cpu().String(),
					CpuLimit:  node.Status.Capacity.Cpu().String(),
					MemoryLimit: node.Status.Capacity.Memory().String(),
					MemoryRequest: node.Status.Allocatable.Memory().String(),
					PodCount: node.Status.Allocatable.Pods().String(),
					PodLimit: node.Status.Capacity.Pods().String(),
				}
				if _,ok := node.Labels["node-role.kubernetes.io/master"]; ok {
					nodeInfo.Type = "master"
				} else {
					nodeInfo.Type = "none"
				}
				for _,condition := range node.Status.Conditions {
					if string(condition.Status) == "True"{
						nodeInfo.Status = string(condition.Type)
					}
				}
				if nodeInfo.Status == ""  {
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