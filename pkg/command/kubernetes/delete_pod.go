package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type DeletePodInfo struct {
	PodName string `json:"podName,omitempty"`
	Namespace string `json:"namespace,omitempty"`
}

func DeletePod(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req DeletePodInfo
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		glog.V(1).Info("[wzl] Unmarshal err: ",err)
		return nil, nil
	}

	namespace := req.Namespace
	podName :=req.PodName
	//vlog.Successf("namespace: %s",namespace)
	err = opts.KubeClient.GetKubeClient().CoreV1().Pods(namespace).Delete(podName, &metav1.DeleteOptions{})
	if err !=nil {
		glog.V(1).Info("[wzl] delete pod err: ",err)
		return nil, &model.Packet{
			Key:     cmd.Key,
			Type:    model.DeletePod,
			Payload:`{"status": "failure"}`,
		}
	}

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.DeletePod,
		Payload:`{"status": "success"}`,
	}
}
