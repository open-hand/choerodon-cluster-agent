package gitops

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/golang/glog"
	"math/rand"
	"strings"
	"sync"
	"time"
)

const SyncIntervalStopChanName = "sync_stop_chan"

type GitOps struct {
	GitHost      string `json:"gitHost,omitempty"`
	Envs         []model.EnvParas
	Wg           *sync.WaitGroup
	stopCh       <-chan struct{}
	syncSoon     map[string]chan struct{}
	gitRepos     map[string]*git.Repo
	syncInterval time.Duration
	gitTimeout   time.Duration
	gitConfig    git.Config
	kubeClient   kube.Client
	cluster      *kubernetes.Cluster
	chans        *channel.CRChan
}

func New(wg *sync.WaitGroup, gitConfig git.Config, gitRepos map[string]*git.Repo, kubeClient kube.Client, cluster *kubernetes.Cluster, chans *channel.CRChan) *GitOps {
	return &GitOps{
		Wg:         wg,
		cluster:    cluster,
		chans:      chans,
		kubeClient: kubeClient,
		gitConfig:  gitConfig,
		syncSoon:   make(map[string]chan struct{}),
		gitRepos:   gitRepos,
	}
}

func (g *GitOps) Process() {
	g.syncInterval = time.Minute * 5
	g.gitTimeout = time.Minute * 1
	g.listenEnvs()
}

func (g *GitOps) WithStop() {
	g.Process()
	// GitStopped置为false
	model.GitRunning = true
}

func (g *GitOps) listenEnvs() {
	rand.Seed(time.Now().Unix())
	if _, ok := model.GitStopChanMap[SyncIntervalStopChanName]; !ok {
		syncIntervalChan := make(chan struct{})
		// 该协程监听环境的定时同步
		go g.SyncInterval(syncIntervalChan)
		model.GitStopChanMap[SyncIntervalStopChanName] = syncIntervalChan
	}

	for _, envPara := range g.Envs {
		if _, ok := model.GitStopChanMap[envPara.Namespace]; !ok {
			gitStopChan := make(chan struct{})
			model.GitStopChanMap[envPara.Namespace] = gitStopChan

			gitRemote := git.Remote{URL: strings.Replace(envPara.GitUrl, g.GitHost, envPara.Namespace, 1)}
			repo := git.NewRepo(gitRemote, envPara.Namespace, git.PollInterval(g.gitConfig.GitPollInterval))
			g.Wg.Add(1)
			// to wait create env git repo
			model.GitRepoConcurrencySyncChan <- struct{}{}
			go func() {
				// repo.Start方法猜测是从gitlab拉取配置文件(注意只拉取.git目录下的文件)
				sleepTime := rand.Intn(60)
				glog.Infof("env: %s will start to sync after %d seconds", envPara.Namespace, sleepTime)
				time.Sleep(time.Duration(sleepTime) * time.Second)
				err := repo.Start(gitStopChan, repo.RefreshChan, g.Wg)
				if err != nil {
					glog.Errorf("git repo start failed", err)
				}
			}()
			g.syncSoon[envPara.Namespace] = make(chan struct{}, 1)
			g.gitRepos[envPara.Namespace] = repo
			g.Wg.Add(1)
			go g.syncLoop(gitStopChan, envPara.Namespace, repo.SyncChan, g.Wg)
		}
	}
}

func (g *GitOps) LogEvent(ev Event, namespace string) error {
	evBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp := &model.Packet{
		Key:     fmt.Sprintf("env:%s", namespace),
		Type:    model.GitOpsSyncEvent,
		Payload: string(evBytes),
	}
	glog.Infof("%s _event:\n%s", resp.Key, resp.Payload)
	g.chans.ResponseChan <- resp
	return nil
}
