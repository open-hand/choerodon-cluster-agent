package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	Funcs.Add(model.GitOpsSync, gitops.DoSync)
}
