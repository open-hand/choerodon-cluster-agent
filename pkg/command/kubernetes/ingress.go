package kubernetes

import (
	"encoding/json"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

func CreateIngress(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	ing, err := opts.KubeClient.CreateOrUpdateIngress(cmd.Namespace(), cmd.Payload)
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

func DeleteIngress(opts *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	err := opts.KubeClient.DeleteIngress(cmd.Namespace(), cmd.Payload)
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
