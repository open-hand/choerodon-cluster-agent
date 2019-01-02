package node

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model/kubernetes"
	"github.com/golang/glog"
	"k8s.io/api/core/v1"
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
			var nodes []string
			nodelist,err := c.lister.List(labels.NewSelector())
			if err != nil {
				glog.Errorf("list node error :", err)
			}
			for _, node := range  nodelist {
				nodes = append(nodes, node.Name)
				c.responseChan <- newNodeRep(node)
			}
			resourceList := &kubernetes.ResourceList{
				Resources:    nodes,
				ResourceType: "Node",
			}
			content, err := json.Marshal(resourceList)
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

func newNodeRep(node *v1.Node) *model.Packet {
	payload, err := json.Marshal(node)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("node:%s", node.Name),
		Type:    model.NodeUpdate,
		Payload: string(payload),
	}
}