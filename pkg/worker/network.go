package worker

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"

	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	registerCmdFunc(model.NetworkService, configureNetworkService)
	registerCmdFunc(model.NetworkIngress, configureNetworkIngress)
	registerCmdFunc(model.NetworkServiceDelete, deleteNetworkService)
	registerCmdFunc(model.NetworkIngressDelete, deleteNetworkIngress)
}

func configureNetworkService(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	svc, err := w.kubeClient.CreateOrUpdateService(cmd.Namespace(), cmd.Payload)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkServiceFailed, err)
	}
	svcB, err := json.Marshal(svc)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkServiceFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.NetworkService,
		Payload: string(svcB),
	}
	return nil, resp
}

func configureNetworkIngress(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	ing, err := w.kubeClient.CreateOrUpdateIngress(cmd.Namespace(), cmd.Payload)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkIngressFailed, err)
	}
	ingB, err := json.Marshal(ing)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkIngressFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.NetworkIngress,
		Payload: string(ingB),
	}
	return nil, resp
}

func deleteNetworkService(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	err := w.kubeClient.DeleteService(cmd.Namespace(), cmd.Payload)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkServiceDeleteFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.NetworkServiceDelete,
		Payload: cmd.Payload,
	}
	return nil, resp
}

func deleteNetworkIngress(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	err := w.kubeClient.DeleteIngress(cmd.Namespace(), cmd.Payload)
	if err != nil {
		return nil, command.NewResponseError(cmd.Key, model.NetworkIngressDeleteFailed, err)
	}
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.NetworkIngressDelete,
		Payload: cmd.Payload,
	}
	return nil, resp
}
