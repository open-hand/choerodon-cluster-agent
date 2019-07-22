package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/agent"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	Funcs.Add(model.InitAgent, agent.InitAgent)
	Funcs.Add(model.CreateEnv, agent.AddEnv)
}
