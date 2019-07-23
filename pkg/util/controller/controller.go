package controller

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
)

type Args struct {
	CrChan       *channel.CRChan
	HelmClient   helm.Client
	Namespaces   *namespace.Namespaces
	KubeClient   kube.Client
	PlatformCode string
}
