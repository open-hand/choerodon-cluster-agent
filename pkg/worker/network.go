package worker

import (
	"encoding/json"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

func init() {
	registerCmdFunc(model.NetworkService, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.configureNetworkService(cmd)
	})
	registerCmdFunc(model.NetworkIngress, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.configureNetworkIngress(cmd)
	})
	registerCmdFunc(model.NetworkServiceDelete, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.deleteNetworkService(cmd)
	})
	registerCmdFunc(model.NetworkIngressDelete, func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool) {
		return w.deleteNetworkIngress(cmd)
	})
}

func (w *workerManager) configureNetworkService(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	svc, err := w.kubeClient.CreateOrUpdateService(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceFailed, err), true
	}
	svcB, err := json.Marshal(svc)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceFailed, err), true
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkService,
		Payload: string(svcB),
	}
	return nil, resp, true
}

func (w *workerManager) configureNetworkIngress(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	ing, err := w.kubeClient.CreateOrUpdateIngress(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressFailed, err), true
	}
	ingB, err := json.Marshal(ing)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressFailed, err), true
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkIngress,
		Payload: string(ingB),
	}
	return nil, resp, true
}

func (w *workerManager) deleteNetworkService(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	err := w.kubeClient.DeleteService(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkServiceDeleteFailed, err), true
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkServiceDelete,
		Payload: cmd.Payload,
	}
	return nil, resp, true
}

func (w *workerManager) deleteNetworkIngress(cmd *model.Command) ([]*model.Command, *model.Response, bool) {
	err := w.kubeClient.DeleteIngress(w.namespace, cmd.Payload)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.NetworkIngressDeleteFailed, err), true
	}
	resp := &model.Response{
		Key:     cmd.Key,
		Type:    model.NetworkIngressDelete,
		Payload: cmd.Payload,
	}
	return nil, resp, true
}
