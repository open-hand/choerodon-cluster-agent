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
	"io"
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
	cluster cluster.Cluster) *workerManager {

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
	}
}

func (w *workerManager) Start(stop <-chan struct{}, wg *sync.WaitGroup) {
	gitconfigChan :=  make(chan  model.GitInitConfig,1 )
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go w.runWorker(i, stop, gitconfigChan, wg)
	}
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

func (w *workerManager) runWorker(i int, stop <-chan struct{}, gitConfig chan <- model.GitInitConfig , done *sync.WaitGroup) {
	defer done.Done()

	for {
		select {
		case <-stop:
			return
		case cmd := <-w.commandChan:
			glog.V(1).Infof("worker %d get command: %s", i, cmd)
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
					break;
				}
				go func(resp *model.Response) {
					w.responseChan <- resp
				}(resp)
			}
		}
	}
}

func registerCmdFunc(funcType string, f processCmdFunc) {
	processCmdFuncs[funcType] = f
}


func writeSSHkey(key string) error {
	path := "/Users/crcokitwood/ssh"
	err :=  os.MkdirAll(path,0777)
	if err != nil {
		return err
	}
	//filename := "/etc/choerodon/ssh/identity"
	filename := "/Users/crcokitwood/ssh/identity"
	var f *os.File
	if checkFileIsExist(filename) { //如果文件存在
		os.Remove(filename)
	}
	f, err = os.Create(filename) //创建文件
	if err != nil {
		return err
	}
	defer f.Close()
	_, err = io.WriteString(f, key) //写入文件(字符串)
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