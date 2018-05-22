package worker

import (
	"encoding/json"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

func init() {
	registerCmdFunc(model.NetworkService, configureNetworkService)
	registerCmdFunc(model.NetworkIngress, configureNetworkIngress)
	registerCmdFunc(model.NetworkServiceDelete, deleteNetworkService)
	registerCmdFunc(model.NetworkIngressDelete, deleteNetworkIngress)
}

func configureNetworkService(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	svc, err := w.kubeClient.CreateOrUpdateService(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceFailed, err)
	}
	svcB, err := json.Marshal(svc)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkService,
		Payload: string(svcB),
	}
	return nil, resp
}

func configureNetworkIngress(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	ing, err := w.kubeClient.CreateOrUpdateIngress(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressFailed, err)
	}
	ingB, err := json.Marshal(ing)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkIngress,
		Payload: string(ingB),
	}
	return nil, resp
}

func deleteNetworkService(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	err := w.kubeClient.DeleteService(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceDeleteFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkServiceDelete,
		Payload: cmd.Payload,
	}
	return nil, resp
}

func deleteNetworkIngress(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	err := w.kubeClient.DeleteIngress(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressDeleteFailed, err)
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkIngressDelete,
		Payload: cmd.Payload,
	}
	return nil, resp
}
