package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/gitops"
)

func init() {
	Funcs.Add(model.GitOpsSync, gitops.DoSync)
}
