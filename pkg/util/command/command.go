package command

import (
	"github.com/choerodon/choerodon-cluster-agent/controller"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"sync"
	"time"
)

type Opts struct {
	GitTimeout        time.Duration
	Namespaces        *manager.Namespaces
	GitRepos          map[string]*git.Repo
	KubeClient        kube.Client
	ControllerContext *controller.ControllerContext
	StopCh            <-chan struct{}
	Cluster           cluster.Cluster
	Wg                *sync.WaitGroup
	CrChan            *manager.CRChan
	GitConfig         git.Config
	Envs              []model.EnvParas `json:"envs,omitempty"`
}
