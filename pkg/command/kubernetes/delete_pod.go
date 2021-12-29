package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeletePodInfo struct {
	PodName   string `json:"podName,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Status    string `json:"status,omitempty"`
}

func DeletePod(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req DeletePodInfo
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		glog.V(1).Info("Unmarshal err: ", err)
		return nil, nil
	}

	namespace := req.Namespace
	podName := req.PodName
	err = opts.KubeClient.GetKubeClient().CoreV1().Pods(namespace).Delete(podName, &metav1.DeleteOptions{})
	if err != nil {
		if errors.IsNotFound(err) {
			return nil, deletePodRespPacket(cmd.Key,"success",&req)
		}
		glog.V(1).Info("Delete pod err: ", err)
		return nil, deletePodRespPacket(cmd.Key, "failed", &req)
	}

	return nil, deletePodRespPacket(cmd.Key, "success", &req)
}

func deletePodRespPacket(key, status string, deletePodInfo *DeletePodInfo) *model.Packet {
	deletePodInfo.Status = status
	payloadByte, _ := json.Marshal(deletePodInfo)
	return &model.Packet{
		Key:     key,
		Type:    model.DeletePod,
		Payload: string(payloadByte),
	}
}
