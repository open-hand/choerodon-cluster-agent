package worker

import (
	"github.com/choerodon/choerodon-agent/pkg/model"
	"encoding/json"
)

func init() {
	registerCmdFunc(model.InitAgent, initAgent)
}


func initAgent(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var req model.GitInitConfig
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    model.InitAgent,
		Payload: cmd.Payload,
	}
}
