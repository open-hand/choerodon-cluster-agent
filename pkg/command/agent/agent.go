package agent

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func InitAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {

	var agentInitOpts model.AgentInitOptions
	err := json.Unmarshal([]byte(cmd.Payload), &agentInitOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	namespaces := opts.Namespaces

	g := gitops.New(opts.Wg, opts.GitConfig, opts.GitRepos, opts.KubeClient, opts.Cluster, opts.CrChan)
	g.GitHost = agentInitOpts.GitHost

	if err := g.PrepareSSHKeys(agentInitOpts.Envs, opts); err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	nsList := []string{}
	for _, envPara := range agentInitOpts.Envs {
		nsList = append(nsList, envPara.Namespace)
		ns, _ := createNamespace(opts.KubeClient, envPara.Namespace)
		if ns == nil {
			glog.V(1).Infof("create namespace %s failed", envPara)
		}
	}
	namespaces.Set(nsList)

	//启动控制器， todo: 后期移除
	opts.ControllerContext.ReSync()

	//启动repo、
	g.Envs = agentInitOpts.Envs
	go g.WithStop(opts.StopCh)

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.InitAgentSucceed,
		Payload: cmd.Payload,
	}

}

func UpgradeAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	upgradeInfo, certInfo, err := opts.HelmClient.ListAgent(cmd.Payload)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.UpgradeClusterFailed, err)
	}
	upgradeInfo.Token = opts.Token
	upgradeInfo.PlatformCode = opts.PlatformCode

	rsp, err := json.Marshal(upgradeInfo)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.UpgradeClusterFailed, err)
	}

	glog.Infof("cluster agent upgrade: %s", string(rsp))
	resp := &model.Packet{
		Key:     cmd.Key,
		Type:    model.Upgrade,
		Payload: string(rsp),
	}

	crChan := opts.CrChan

	if certInfo != nil {
		certRsp, err := json.Marshal(certInfo)
		if err != nil {
			glog.Errorf("check cert manager error while marshal cert rsp")
		} else {
			certInfoResp := &model.Packet{
				Key:     cmd.Key,
				Type:    model.CertManagerInfo,
				Payload: string(certRsp),
			}
			crChan.ResponseChan <- certInfoResp
		}

	} else {
		certInfoResp := &model.Packet{
			Key:  cmd.Key,
			Type: model.CertManagerInfo,
		}
		crChan.ResponseChan <- certInfoResp
	}
	return nil, resp
}

func ReSyncAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	fmt.Println("get command re_sync")
	opts.ControllerContext.ReSync()
	return nil, nil
}

func createNamespace(kubeClient kube.Client, namespace string) (*v1.Namespace, error) {
	return kubeClient.GetKubeClient().CoreV1().Namespaces().Create(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	})
}
