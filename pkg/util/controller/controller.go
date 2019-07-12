package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
)

type Args struct {
	CrChan     *manager.CRChan
	HelmClient helm.Client
}
