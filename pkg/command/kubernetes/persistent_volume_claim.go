package kubernetes

import (
	"context"
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

type DeletePvcInfo struct {
	Labels    map[string]string
	Namespace string
}

func DeletePvcByLabels(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req DeletePvcInfo
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		glog.V(1).Info("Unmarshal err: ", err)
		return nil, nil
	}

	labelPvc := labels.SelectorFromSet(labels.Set(map[string]string{"app": req.Labels["app"]}))
	listPvcOptions := metav1.ListOptions{
		LabelSelector: labelPvc.String(),
	}
	err = opts.KubeClient.GetKubeClient().CoreV1().PersistentVolumeClaims(req.Namespace).DeleteCollection(context.TODO(),metav1.DeleteOptions{}, listPvcOptions)
	if err != nil {
		glog.Info("Delete Pvc err: ", err.Error())
		return nil, nil
	}

	return nil, nil
}
