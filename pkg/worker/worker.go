package worker

import (
	"fmt"
	"sync"

	"github.com/golang/glog"

	"time"

	"github.com/choerodon/choerodon-agent/pkg/appclient"
	"github.com/choerodon/choerodon-agent/pkg/cluster"
	"github.com/choerodon/choerodon-agent/pkg/git"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
	"encoding/json"
	"os"
	"io/ioutil"
)

var (
	processCmdFuncs = make(map[string]processCmdFunc)
)

type processCmdFunc func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response)

type workerManager struct {
	commandChan  chan *model.Command
	responseChan chan<- *model.Response
	helmClient   helm.Client
	kubeClient   kube.Client
	appClient    appclient.AppClient
	namespace    string
	gitConfig    git.Config
	gitRepo      *git.Repo
	syncSoon     chan struct{}
	syncInterval time.Duration
	manifests    cluster.Manifests
	cluster      cluster.Cluster
	statusSyncInterval   time.Duration
}

func NewWorkerManager(
	commandChan chan *model.Command,
	responseChan chan<- *model.Response,
	kubeClient kube.Client,
	helmClient helm.Client,
	appClient appclient.AppClient,
	namespace string,
	gitConfig git.Config,
	gitRepo *git.Repo,
	syncInterval time.Duration,
	manifests cluster.Manifests,
	cluster cluster.Cluster,
	statusSyncInterval time.Duration) *workerManager {

	return &workerManager{
		commandChan:  commandChan,
		responseChan: responseChan,
		helmClient:   helmClient,
		kubeClient:   kubeClient,
		appClient:    appClient,
		namespace:    namespace,
		gitConfig:    gitConfig,
		gitRepo:      gitRepo,
		syncSoon:     make(chan struct{}, 1),
		syncInterval: syncInterval,
		manifests:    manifests,
		cluster:      cluster,
		statusSyncInterval: statusSyncInterval,
	}
}

func (w *workerManager) Start(stop <-chan struct{}, wg *sync.WaitGroup) {
	wg.Add(1)
	go w.syncStatus(stop, wg)
	gitconfigChan :=  make(chan  model.GitInitConfig,1 )

		wg.Add(1)
		go w.runWorker( stop, gitconfigChan, wg)

	if w.gitConfig.GitUrl == "" {
		for {
			gitConfig := <- gitconfigChan
			gitRemote := git.Remote{URL: gitConfig.GitUrl}
			glog.Infof("receive init git url %s and git ssh key :\n%s",gitConfig.GitUrl,gitConfig.SshKey)
			if err := writeSSHkey(gitConfig.SshKey); err != nil {
				glog.Errorf("write git ssh key error",err	)
			}else {
				glog.Info("Init Git Config Success")
				w.gitRepo = git.NewRepo(gitRemote, git.PollInterval(w.gitConfig.GitPollInterval))

				{
					wg.Add(1)
					go func() {
						err := w.gitRepo.Start(stop, wg)
						if err != nil {
							glog.Errorf("git repo start failed", err)
						}
					}()
				}
				break
			}
		}
	}
	wg.Add(1)
	go w.syncLoop(stop, wg)
}

func (w *workerManager) runWorker(stop <-chan struct{}, gitConfig chan <- model.GitInitConfig , done *sync.WaitGroup) {
	defer done.Done()
	for {
		select {
			case <-stop:
				glog.Infof("worker down!")
				return
			case cmd := <-w.commandChan:
				go func(cmd *model.Command) {
					glog.Infof("get command: %s/%s",  cmd.Key, cmd.Type)
					var newCmds []*model.Command = nil
					var resp *model.Response = nil

					if processCmdFunc, ok := processCmdFuncs[cmd.Type]; !ok {
						err := fmt.Errorf("type %s not exist", cmd.Type)
						glog.V(2).Info(err.Error())
						resp = NewResponseError(cmd.Key, cmd.Type, err)
					} else {
						newCmds, resp = processCmdFunc(w, cmd)
					}

					if newCmds != nil {
						go func(newCmds []*model.Command) {
							for i := 0; i < len(newCmds); i++ {
								w.commandChan <- newCmds[i]
							}
						}(newCmds)
					}
					if resp != nil {
						if resp.Type == model.InitAgent{
							var config model.GitInitConfig
							err := json.Unmarshal([]byte(resp.Payload), &config)
							if err != nil {
								glog.Errorf("unmarshal git config error", err)
							}
							gitConfig <- config
							return
						}
						go func(resp *model.Response) {
							w.responseChan <- resp
						}(resp)
					}
				}(cmd)
			}
	}
}

func registerCmdFunc(funcType string, f processCmdFunc) {
	processCmdFuncs[funcType] = f
}


func writeSSHkey(key string) error {
	path := "/etc/choerodon/ssh"
	err :=  os.MkdirAll(path,0777)
	if err != nil {
		return err
	}
	filename := path+"/identity"
	var f *os.File
	if checkFileIsExist(filename) { //如果文件存在
		os.Remove(filename)
	}
	f, err = os.OpenFile(filename, os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	f.Close()
	 err = ioutil.WriteFile(filename,[]byte(key),0600) //写入文件(字符串)
	if  err != nil {
		return err;
	}
	return nil
}

func checkFileIsExist(filename string) bool {
	var exist = true
	if _, err := os.Stat(filename); os.IsNotExist(err) {
		exist = false
	}
	return exist
}