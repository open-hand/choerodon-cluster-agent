package worker

import (
	"fmt"

	"github.com/golang/glog"

	"github.com/choerodon/choerodon-agent/pkg/appclient"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
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
}

func (w *workerManager) Start() {
	for i := 0; i < 5; i++ {
		go w.runWorker(i)
	}
}

func (w *workerManager) runWorker(i int) {
	for cmd := range w.commandChan {
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
