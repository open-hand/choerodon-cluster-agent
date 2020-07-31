package agent

import (
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	agentsync "github.com/choerodon/choerodon-cluster-agent/pkg/agent/sync"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/config"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	operatorutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/operator"
	"sync"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"github.com/golang/glog"
	vlog "github.com/vinkdong/gox/log"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"time"
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
	polarisConfig      *config.Configuration
	clearHelmHistory   bool
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
	syncAll bool,
	polarisConfig *config.Configuration,
	clearHelmHistory bool) *workerManager {
	return &workerManager{
		chans:              chans,
		helmClient:         helmClient,
		kubeClient:         kubeClient,
		appClient:          appClient,
		agentInitOps:       agentInitOps,
		syncInterval:       syncInterval,
		statusSyncInterval: statusSyncInterval,
		gitTimeout:         gitTimeout,
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
		polarisConfig:      polarisConfig,
		clearHelmHistory:   clearHelmHistory,
	}
}

func (w *workerManager) Start() {
	w.wg.Add(1)
	go w.runWorker()
	go w.monitorCertMgr()

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
						AgentInitOps:      w.agentInitOps,
						HelmClient:        w.helmClient,
						PlatformCode:      w.platformCode,
						WsClient:          w.appClient,
						Token:             w.token,
						Mgrs:              w.mgrs,
						PolarisConfig:     w.polarisConfig,
						ClearHelmHistory:  w.clearHelmHistory,
					}
					newCmds, resp = processCmdFunc(opts, cmd)
				} else {
					err := fmt.Errorf("type %s not exist", cmd.Type)
					glog.Info(err.Error())
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

//监听cert-mgr的pod运行情况
func (w *workerManager) monitorCertMgr() {

	//临时存储pod status 每过10s判断pod状态是否更改，若更改则发送消息。
	podStatusTmp := " "
	for {
		time.Sleep(10 * time.Second)
		podStatus, err := w.getPodStatus()
		if err != nil {
			return
		}
		if podStatus != podStatusTmp {
			podStatusTmp = podStatus
			w.chans.ResponseChan <- &model.Packet{
				Key:     "cluster:" + kube.ClusterId,
				Type:    model.CertManagerStatus,
				Payload: fmt.Sprintf(model.PodStatus, podStatus),
			}
		}
	}
}

//得到pod的状态
func (w *workerManager) getPodStatus() (string, error) {
	//为了兼容老版本
	podListOld, err := w.kubeClient.GetKubeClient().CoreV1().Pods("kube-system").List(metav1.ListOptions{
		LabelSelector: "choerodon.io/release=choerodon-cert-manager",
	})
	if err != nil {
		glog.V(1).Info("Namespace kube-system dont have the cert-mgr")
		return "exception", err
	}

	if len(podListOld.Items) > 0 {
		glog.V(1).Info("The old cert-mgr pod  is running in the kube-system")
		return "running", nil
	}
	//新版本从这开始--
	//这个方法从找podList开始，原因是： 如果pod直接被删除，那么名字会更换
	const namespace = "choerodon"
	podList, err := w.kubeClient.GetKubeClient().CoreV1().Pods(namespace).List(metav1.ListOptions{
		LabelSelector: "choerodon.io/release=choerodon-cert-manager",
	})
	if err != nil {
		glog.V(1).Info("Get cert-mgr pod by selector err: ", err)
		return "exception", err
	}
	if len(podList.Items) == 0 {
		glog.V(1).Info("The cert-mgr pod status is deleted")
		return "deleted", err
	}
	if len(podList.Items) > 1 {
		glog.V(1).Info("The cert-mgr pod got by selector Is not the only")
		return "", err
	}

	podName := podList.Items[0].Name
	pod, err := w.kubeClient.GetKubeClient().CoreV1().Pods(namespace).Get(podName, metav1.GetOptions{})
	if err != nil {
		glog.V(1).Info("Get pod status by podName err: ", err)
		return "", err
	}
	podStatus := fmt.Sprintf("%s", pod.Status.Phase)
	return podStatus, nil
}
