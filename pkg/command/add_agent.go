package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/agent"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	Funcs.Add(model.InitAgent, agent.InitAgent)
	Funcs.Add(model.UpgradeCluster, agent.UpgradeAgent)

	Funcs.Add(model.ReSyncAgent, agent.ReSyncAgent)

	Funcs.Add(model.CreateEnv, agent.AddEnv)
	Funcs.Add(model.EnvDelete, agent.DeleteEnv)
}
