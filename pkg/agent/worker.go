package agent

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	agentsync "github.com/choerodon/choerodon-cluster-agent/pkg/agent/sync"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	operatorutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/operator"
	"sync"

	"github.com/golang/glog"
	"time"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	vlog "github.com/vinkdong/gox/log"
)

type WorkerManager workerManager

type workerManager struct {
	chans              *channel.CRChan
	clusterId          int
	helmClient         helm.Client
	kubeClient         kube.Client
	appClient          websocket.Client
	agentInitOps       *model.AgentInitOptions
	gitConfig          git.Config
	gitRepos           map[string]*git.Repo
	syncSoon           map[string]chan struct{}
	syncInterval       time.Duration
	cluster            *kubernetes.Cluster
	statusSyncInterval time.Duration
	gitTimeout         time.Duration
	controllerContext  *agentsync.Context
	wg                 *sync.WaitGroup
	stop               <-chan struct{}
	token              string
	platformCode       string
	syncAll            bool
	mgrs               *operatorutil.MgrList
}

func NewWorkerManager(
	mgrs *operatorutil.MgrList,
	chans *channel.CRChan,
	kubeClient kube.Client,
	helmClient helm.Client,
	appClient websocket.Client,
	cluster *kubernetes.Cluster,
	agentInitOps *model.AgentInitOptions,
	syncInterval time.Duration,
	statusSyncInterval time.Duration,
	gitTimeout time.Duration,
	gitConfig git.Config,
	controllerContext *agentsync.Context,
	wg *sync.WaitGroup,
	stop <-chan struct{},
	token string,
	platformCode string,
	syncAll bool) *workerManager {
	return &workerManager{
		chans:              chans,
		helmClient:         helmClient,
		kubeClient:         kubeClient,
		appClient:          appClient,
		agentInitOps:       agentInitOps,
		syncInterval:       syncInterval,
		statusSyncInterval: statusSyncInterval,
		gitTimeout:         gitTimeout,
		gitRepos:           map[string]*git.Repo{},
		gitConfig:          gitConfig,
		syncSoon:           map[string]chan struct{}{},
		wg:                 wg,
		stop:               stop,
		controllerContext:  controllerContext,
		cluster:            cluster,
		token:              token,
		platformCode:       platformCode,
		syncAll:            syncAll,
		mgrs:               mgrs,
	}
}

func (w *workerManager) Start() {
	w.wg.Add(1)
	go w.runWorker()
}

func (w *workerManager) runWorker() {
	defer w.wg.Done()
	for {
		select {
		case <-w.stop:
			glog.Infof("worker down!")
			return
		case cmd := <-w.chans.CommandChan:
			go func(cmd *model.Packet) {
				if cmd == nil {
					glog.Error("got wrong command")
					return
				}
				vlog.Successf("get command: %s/%s", cmd.Key, cmd.Type)
				var newCmds []*model.Packet = nil
				var resp *model.Packet = nil

				if processCmdFunc, ok := command.Funcs[cmd.Type]; ok {
					opts := &commandutil.Opts{
						GitTimeout:        w.gitTimeout,
						Namespaces:        w.controllerContext.Namespaces,
						GitRepos:          w.gitRepos,
						KubeClient:        w.kubeClient,
						ControllerContext: w.controllerContext,
						StopCh:            w.stop,
						Cluster:           w.cluster,
						Wg:                w.wg,
						CrChan:            w.chans,
						GitConfig:         w.gitConfig,
						Envs:              w.agentInitOps.Envs,
						HelmClient:        w.helmClient,
						PlatformCode:      w.platformCode,
						WsClient:          w.appClient,
						Token:             w.token,
						Mgrs:              w.mgrs,
					}
					newCmds, resp = processCmdFunc(opts, cmd)
				} else {
					err := fmt.Errorf("type %s not exist", cmd.Type)
					glog.V(1).Info(err.Error())
				}

				if newCmds != nil {
					go func(newCmds []*model.Packet) {
						for i := 0; i < len(newCmds); i++ {
							w.chans.CommandChan <- newCmds[i]
						}
					}(newCmds)
				}
				if resp != nil {
					go func(resp *model.Packet) {
						w.chans.ResponseChan <- resp
					}(resp)
				}
			}(cmd)
		}
	}
}
