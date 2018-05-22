package worker

import (
	"github.com/golang/glog"

	"github.com/choerodon/choerodon-agent/pkg/appclient"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
)

const (
	retryTimes uint = 0
)

var (
	processCmdFuncs = make(map[string]processCmdFunc)
)

type processCmdFunc func(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response, bool)

type workerManager struct {
	commandChan  chan *model.Command
	responseChan chan<- *model.Response
	helmClient   helm.Client
	kubeClient   kube.Client
	appClient    appclient.AppClient
	namespace    string
}

func (w *workerManager) Start() {
	for i := 0; i < 5; i++ {
		go w.runWorker(i)
	}
}

func (w *workerManager) runWorker(i int) {
	for cmd := range w.commandChan {
		glog.V(1).Infof("worker %d get command: %s", i, cmd)
		var forget = false
		var newCmds []*model.Command = nil
		var resp *model.Response = nil

		if processCmdFunc, ok := processCmdFuncs[cmd.Type]; !ok {
			glog.V(2).Infof("type %s not exist", cmd.Type)
			continue
		} else {
			newCmds, resp, forget = processCmdFunc(w, cmd)
		}

		if newCmds != nil {
			go func(newCmds []*model.Command) {
				for i := 0; i < len(newCmds); i++ {
					w.commandChan <- newCmds[i]
				}
			}(newCmds)
		}
		if !forget && cmd.Retry < retryTimes {
			go func(cmd *model.Command) {
				cmd.Retry = cmd.Retry + 1
				w.commandChan <- cmd
			}(cmd)
		}
		if resp != nil {
			go func(resp *model.Response) {
				w.responseChan <- resp
			}(resp)
		}
	}
}

func registerCmdFunc(funcType string, f processCmdFunc) {
	processCmdFuncs[funcType] = f
}

func NewWorkerManager(
	commandChan chan *model.Command,
	responseChan chan<- *model.Response,
	kubeClient kube.Client,
	helmClient helm.Client,
	appClient appclient.AppClient,
	namespace string) *workerManager {

	return &workerManager{
		commandChan:  commandChan,
		responseChan: responseChan,
		helmClient:   helmClient,
		kubeClient:   kubeClient,
		appClient:    appClient,
		namespace:    namespace,
	}
}
