package controller

import "github.com/choerodon/choerodon-cluster-agent/pkg/controller/persistentvolumeclaims"

func init() {
	// AddToManagerFuncs is a list of functions to create controllers and add them to a manager.
	AddToManagerFuncs = append(AddToManagerFuncs, persistentvolumeclaims.Add)
}
