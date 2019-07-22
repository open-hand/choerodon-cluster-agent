package worker

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	registerCmdFunc(model.ReSyncAgent, reSync)
	registerCmdFunc(model.UpgradeCluster, upgrade)
	registerCmdFunc(model.EnvDelete, deleteEnv)
}
