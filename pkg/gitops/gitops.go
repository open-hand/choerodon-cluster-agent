package gitops

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/event"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/golang/glog"
	"strings"
	"sync"
	"time"
)

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
	cluster      cluster.Cluster
	chans        *manager.CRChan
}

func New(wg *sync.WaitGroup, gitConfig git.Config, gitRepos map[string]*git.Repo, kubeClient kube.Client, cluster cluster.Cluster, chans *manager.CRChan) *GitOps {
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
	// todo read from config
	g.syncInterval = time.Minute * 5
	g.gitTimeout = time.Minute * 1
	g.listenEnvs()
}

func (g *GitOps) WithStop(stopCh <-chan struct{}) {
	g.stopCh = stopCh
	g.Process()
}

func (g *GitOps) listenEnvs() {
	for _, envPara := range g.Envs {
		gitRemote := git.Remote{URL: strings.Replace(envPara.GitUrl, g.GitHost, envPara.Namespace, 1)}
		repo := git.NewRepo(gitRemote, envPara.Namespace, git.PollInterval(g.gitConfig.GitPollInterval))
		g.Wg.Add(1)
		repoStopChan := make(chan struct{}, 1)
		// to wait create env git repo
		time.Sleep(10 * time.Second)
		go func() {
			err := repo.Start(g.stopCh, repoStopChan, g.Wg)
			if err != nil {
				glog.Errorf("git repo start failed", err)
			}
		}()
		g.syncSoon[envPara.Namespace] = make(chan struct{}, 1)
		g.gitRepos[envPara.Namespace] = repo
		g.Wg.Add(1)
		go g.syncLoop(g.stopCh, envPara.Namespace, repoStopChan, g.Wg)
	}
}

func (g *GitOps) LogEvent(ev event.Event, namespace string) error {
	evBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp := &model.Packet{
		Key:     fmt.Sprintf("env:%s", namespace),
		Type:    model.GitOpsSyncEvent,
		Payload: string(evBytes),
	}
	glog.Infof("%s git_ops_sync_event:\n%s", resp.Key, resp.Payload)
	g.chans.ResponseChan <- resp
	return nil
}
