package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

func init() {
	Funcs.Add(model.KubernetesGetLogs, kubernetes.LogsByKubernetes)
	Funcs.Add(model.KubernetesExec, kubernetes.ExecByKubernetes)
	Funcs.Add(model.OperatePodCount, kubernetes.ScalePod)
}
