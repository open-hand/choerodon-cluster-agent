package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

func CreateService(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	svc, err := opts.KubeClient.CreateOrUpdateService(cmd.Namespace(), cmd.Payload)
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

func DeleteService(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	err := opts.KubeClient.DeleteService(cmd.Namespace(), cmd.Payload)
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
