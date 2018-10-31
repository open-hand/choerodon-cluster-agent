package worker

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"strings"
	"sync"

	"github.com/golang/glog"

	"time"

	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/ws"
)

var (
	processCmdFuncs = make(map[string]processCmdFunc)
)

type processCmdFunc func(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet)

type workerManager struct {
	chans              *manager.CRChan
	clusterId            int
	helmClient         helm.Client
	kubeClient         kube.Client
	appClient          ws.WebSocketClient
	agentInitOps       *model.AgentInitOptions
	gitConfig          git.Config
	gitRepos           map[string]*git.Repo
	syncSoon           map[string]chan struct{}
	syncInterval       time.Duration
	manifests          cluster.Manifests
	cluster            cluster.Cluster
	statusSyncInterval time.Duration
	gitTimeout         time.Duration
	agentInitOpsChan   chan model.AgentInitOptions
}

type opsRepos struct {
}

func NewWorkerManager(
	chans *manager.CRChan,
	kubeClient kube.Client,
	helmClient helm.Client,
	appClient ws.WebSocketClient,
	agentInitOps *model.AgentInitOptions,
	syncInterval time.Duration,
	statusSyncInterval time.Duration,
	gitTimeout time.Duration) *workerManager {

	return &workerManager{
		chans:              chans,
		helmClient:         helmClient,
		kubeClient:         kubeClient,
		appClient:          appClient,
		agentInitOps:       agentInitOps,
		agentInitOpsChan:   make(chan model.AgentInitOptions, 1),
		syncInterval:       syncInterval,
		statusSyncInterval: statusSyncInterval,
		gitTimeout:         gitTimeout,
		gitRepos:           map[string]*git.Repo{},
	}
}

func (w *workerManager) Start(stop <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go w.syncStatus(stop, wg)

	//gitconfigChan := make(chan model.GitInitConfig, 1)

	wg.Add(1)
	go w.runWorker(stop, wg)

	//if w.gitConfig.GitUrl == "" {
	//	for {
	//		gitConfig := <-gitconfigChan
	//		gitRemote := git.Remote{URL: gitConfig.GitUrl}
	//		glog.Infof("receive manager git url %s and git ssh key :\n%s", gitConfig.GitUrl, gitConfig.SshKey)
	//		if err := writeSSHkey(gitConfig.SshKey); err != nil {
	//			glog.Errorf("write git ssh key error", err)
	//		} else {
	//			glog.Info("Init Git Config Success")
	//			w.gitRepo = git.NewRepo(gitRemote, git.PollInterval(w.gitConfig.GitPollInterval))
	//
	//			{
	//				wg.Add(1)
	//				go func() {
	//					err := w.gitRepo.Start(stop, wg)
	//					if err != nil {
	//						glog.Errorf("git repo start failed", err)
	//					}
	//				}()
	//			}
	//			break
	//		}
	//	}
	//}

	w.startRepos(stop, wg)

}

func (w *workerManager) runWorker(stop <-chan struct{}, done *sync.WaitGroup) {
	defer done.Done()
	for {
		select {
		case <-stop:
			glog.Infof("worker down!")
			return
		case cmd := <-w.chans.CommandChan:
			go func(cmd *model.Packet) {
				glog.Infof("get command: %s/%s", cmd.Key, cmd.Type)
				var newCmds []*model.Packet = nil
				var resp *model.Packet = nil

				if processCmdFunc, ok := processCmdFuncs[cmd.Type]; !ok {
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
					if resp.Type == model.InitAgentSucceed {
						var envParas model.AgentInitOptions
						err := json.Unmarshal([]byte(resp.Payload), &envParas)
						if err != nil {
							glog.Errorf("unmarshal git config error", err)
						}
						w.agentInitOpsChan <- envParas
						return
					}
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

func (w *workerManager) startRepos(stop <-chan struct{}, wg *sync.WaitGroup) {
	repos := <-w.agentInitOpsChan
	for _, envPara := range repos.Envs {
		strings.Replace(envPara.GitUrl, repos.GitHost, envPara.Namespace, 1)
		gitRemote := git.Remote{URL: envPara.GitUrl}
		repo := git.NewRepo(gitRemote, git.PollInterval(w.gitConfig.GitPollInterval))
		wg.Add(1)
		go func() {
			err := repo.Start(stop, wg)
			if err != nil {
				glog.Errorf("git repo start failed", err)
			}
		}()
		w.gitRepos[envPara.Namespace] = repo
		repoStopChan := make(chan<- struct{}, 1)
		wg.Add(1)
		go w.syncLoop(stop, envPara.Namespace, repoStopChan, wg)
	}

}

func (w *workerManager) initAgent() {
	// 往文件中写入各个git库deploy key
	w.syncSoon = map[string]chan struct{}{}
	for _, envPara := range w.agentInitOps.Envs {
		repoSyncSoon := make(chan struct{}, 1)
		w.syncSoon[envPara.Namespace] = repoSyncSoon
	}

}
