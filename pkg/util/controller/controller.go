package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
)

type Args struct {
	CrChan       *manager.CRChan
	HelmClient   helm.Client
	Namespaces   *manager.Namespaces
	KubeClient   kube.Client
	PlatformCode string
}
