package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/controller/ingress/v1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/controller/ingress/v1beta1"
)

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, v1.Add)
	AddToManagerFuncs = append(AddToManagerFuncs, v1beta1.Add)
}
