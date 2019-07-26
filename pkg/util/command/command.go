package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	agentsync "github.com/choerodon/choerodon-cluster-agent/pkg/agent/sync"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"sync"
	"time"
)

type Opts struct {
	GitTimeout        time.Duration
	Namespaces        *namespace.Namespaces
	GitRepos          map[string]*git.Repo
	KubeClient        kube.Client
	ControllerContext *agentsync.Context
	StopCh            <-chan struct{}
	Cluster           *kubernetes.Cluster
	Wg                *sync.WaitGroup
	CrChan            *channel.CRChan
	GitConfig         git.Config
	Envs              []model.EnvParas `json:"envs,omitempty"`
	HelmClient        helm.Client
	PlatformCode      string
	WsClient          websocket.Client
	Token             string
}
