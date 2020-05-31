package packet

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
)

func NamespacePacket(namespaces *v1.NamespaceList) *model.Packet {
	var namespaceList []string
	for _, v := range namespaces.Items {
		namespaceList = append(namespaceList, v.Name)
	}
	payload, err := json.Marshal(namespaceList)
	if err != nil {
		glog.Error(err)
	}
	return &model.Packet{
		Key:     fmt.Sprintf("cluster:%d", kube.ClusterId),
		Type:    "namespace_info",
		Payload: string(payload),
	}
}
