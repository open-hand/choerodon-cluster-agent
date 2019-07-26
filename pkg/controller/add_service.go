package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/controller/service"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, service.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, service.AddServicePod)
}
