package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/polaris"
)

func init() {
	Funcs.Add(model.PolarisRequest, polaris.ScanSystem)
}
