package kubernetes

import (
	"context"
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type ScalePodRequest struct {
	Name      string `json:"name,omitempty"`
	Count     int    `json:"count,omitempty"`
	Namespace string `json:"namespace,omitempty"`
	Kind      string `json:"kind,omitempty"`
	CommandId string `json:"commandId,omitempty"`
}

type ScalePodResponse struct {
	CommandId string `json:"commandId,omitempty"`
	Msg       string `json:"msg,omitempty"`
}

func ScalePod(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req *ScalePodRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, err)
	}
	switch req.Kind {
	case "Deployment":
		clientSet := opts.KubeClient.GetKubeClient()
		s, err := clientSet.AppsV1().Deployments(req.Namespace).GetScale(context.TODO(), req.Name, metav1.GetOptions{})
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, err)
		}

		s.Spec.Replicas = int32(req.Count)

		_, err = clientSet.AppsV1().Deployments(req.Namespace).UpdateScale(context.TODO(), req.Name, s, metav1.UpdateOptions{})
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, err)
		}
	case "StatefulSet":
		clientSet := opts.KubeClient.GetKubeClient()
		s, err := clientSet.AppsV1().StatefulSets(req.Namespace).GetScale(context.TODO(), req.Name, metav1.GetOptions{})
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, err)
		}

		s.Spec.Replicas = int32(req.Count)

		_, err = clientSet.AppsV1().StatefulSets(req.Namespace).UpdateScale(context.TODO(), req.Name, s, metav1.UpdateOptions{})
		if err != nil {
			return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, err)
		}
	default:
		return nil, NewResponseError(cmd.Key, model.OperatePodCountFailed, req.CommandId, errors.New("unsupported resource kind"))

	}

	response := &ScalePodResponse{
		CommandId: req.CommandId,
	}
	jsonByte, _ := json.Marshal(response)

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.OperatePodCountSuccess,
		Payload: string(jsonByte),
	}
}

func NewResponseError(key, cmdType, commandId string, err error) *model.Packet {
	glog.Error(err)
	response := &ScalePodResponse{
		CommandId: commandId,
		Msg:       err.Error(),
	}
	jsonByte, _ := json.Marshal(response)
	return &model.Packet{
		Key:     key,
		Type:    cmdType,
		Payload: string(jsonByte),
	}
}
