package worker

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/controller"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"sync"

	"github.com/golang/glog"
	"time"

	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
)

var (
	processCmdFuncs = make(map[string]processCmdFunc)
)

type processCmdFunc func(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet)

type WorkerManager workerManager

type workerManager struct {
	chans              *manager.CRChan
	clusterId          int
	helmClient         helm.Client
	kubeClient         kube.Client
	appClient          websocket.Client
	agentInitOps       *model.AgentInitOptions
	gitConfig          git.Config
	gitRepos           map[string]*git.Repo
	syncSoon           map[string]chan struct{}
	repoStopChans      map[string]chan struct{}
	syncInterval       time.Duration
	manifests          cluster.Manifests
	cluster            cluster.Cluster
	statusSyncInterval time.Duration
	gitTimeout         time.Duration
	controllerContext  *controller.ControllerContext
	wg                 *sync.WaitGroup
	stop               <-chan struct{}
	token              string
	platformCode       string
	syncAll            bool
}

func NewWorkerManager(
	chans *manager.CRChan,
	kubeClient kube.Client,
	helmClient helm.Client,
	appClient websocket.Client,
	manifests cluster.Manifests,
	cluster cluster.Cluster,
	agentInitOps *model.AgentInitOptions,
	syncInterval time.Duration,
	statusSyncInterval time.Duration,
	gitTimeout time.Duration,
	gitConfig git.Config,
	controllerContext *controller.ControllerContext,
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
		repoStopChans:      map[string]chan struct{}{},
		wg:                 wg,
		stop:               stop,
		controllerContext:  controllerContext,
		manifests:          manifests,
		cluster:            cluster,
		token:              token,
		platformCode:       platformCode,
		syncAll:            syncAll,
	}
}

func (w *workerManager) Start() {
	w.wg.Add(1)
	go w.syncStatus()

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
				glog.Infof("get command: %s/%s", cmd.Key, cmd.Type)
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
					}
					newCmds, resp = processCmdFunc(opts, cmd)
					// todo: remove else when all command moved
				} else if processCmdFunc, ok := processCmdFuncs[cmd.Type]; !ok {
					err := fmt.Errorf("type %s not exist", cmd.Type)
					glog.V(1).Info(err.Error())
				} else {
					newCmds, resp = processCmdFunc(w, cmd)
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

func registerCmdFunc(funcType string, f processCmdFunc) {
	processCmdFuncs[funcType] = f
}

func deleteEnv(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var env model.EnvParas
	err := json.Unmarshal([]byte(cmd.Payload), &env)

	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvDelete, err)
	}

	if !w.controllerContext.Namespaces.Contain(env.Namespace) {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvDelete, err)
	}

	w.controllerContext.Namespaces.Remove(env.Namespace)

	newEnvs := []model.EnvParas{}

	for index, envPara := range w.agentInitOps.Envs {

		if envPara.Namespace == env.Namespace {
			newEnvs = append(w.agentInitOps.Envs[0:index], w.agentInitOps.Envs[index+1:]...)
		}

	}
	w.agentInitOps.Envs = newEnvs

	w.helmClient.DeleteNamespaceReleases(env.Namespace)
	w.kubeClient.DeleteNamespace(env.Namespace)
	close(w.repoStopChans[env.Namespace])

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.EnvDeleteSucceed,
		Payload: cmd.Payload,
	}

}

func reSync(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	fmt.Println("get command re_sync")
	w.controllerContext.ReSync()
	return nil, nil
}
