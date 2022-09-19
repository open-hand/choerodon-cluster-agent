package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	helmcommon "github.com/choerodon/choerodon-cluster-agent/pkg/command/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/operator"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
	"github.com/golang/glog"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"time"
)

func InitAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	model.InitLock.Lock()
	defer func() {
		model.InitLock.Unlock()
		model.Initialized = true
	}()

	if model.Initialized == true {
		glog.Info("agent already initialized")
		return getClusterInfo(opts, cmd)
	}

	var agentInitOpts model.AgentInitOptions
	err := json.Unmarshal([]byte(cmd.Payload), &agentInitOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	// 如果devops没有设置该参数，那么默认设为1
	if agentInitOpts.RepoConcurrencySyncSize == 0 {
		agentInitOpts.RepoConcurrencySyncSize = 1
	}

	glog.Infof("repoConcurrencySize:%d", agentInitOpts.RepoConcurrencySyncSize)

	model.GitRepoConcurrencySyncChan = make(chan struct{}, agentInitOpts.RepoConcurrencySyncSize)

	model.CertManagerVersion = agentInitOpts.CertManagerVersion

	namespaces := opts.Namespaces

	g := gitops.New(opts.Wg, opts.GitConfig, opts.GitRepos, opts.KubeClient, opts.Cluster, opts.CrChan)
	g.GitHost = agentInitOpts.GitHost

	if err := g.PrepareSSHKeys(agentInitOpts.Envs, opts); err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	// 此处naList不能使用make进行初始化，比如make([]string,10),make初始化的结果会包含空串，空串会导致创建监听整个集群的controller
	nsList := []string{}
	// 检查devops管理的命名空间
	for _, envPara := range agentInitOpts.Envs {
		nsList = append(nsList, envPara.Namespace)
		err := createNamespace(opts, envPara.Namespace, envPara.Releases)
		if err != nil {
			return nil, commandutil.NewResponseError(cmd.Key, cmd.Type, err)
		}
	}
	namespaces.Set(nsList)

	//里面含有好多 启动时的方法， 比如启动时发送cert-mgr的情况
	opts.ControllerContext.ReSync()

	cfg, err := opts.KubeClient.GetRESTConfig()
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	args := &controller.Args{
		CrChan:       opts.CrChan,
		HelmClient:   opts.HelmClient,
		Namespaces:   namespaces,
		KubeClient:   opts.KubeClient,
		PlatformCode: opts.PlatformCode,
	}

	for _, ns := range nsList {
		currentNamespace := ns
		if opts.Mgrs.IsExist(currentNamespace) {
			continue
		}
		mgr, err := operator.New(cfg, currentNamespace, args)
		if err != nil {
			return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
		}
		// check success added avoid repeat watch
		if opts.Mgrs.AddStop(currentNamespace, mgr) {
			go func() {
				if err := mgr.Start(opts.Mgrs.GetCtx(currentNamespace)); err != nil {
					opts.CrChan.ResponseChan <- commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
				}
			}()
		}
	}

	g.Envs = agentInitOpts.Envs
	opts.AgentInitOps.Envs = agentInitOpts.Envs
	go g.WithStop()

	return returnInitResult(opts, cmd)
}

// 以前用于重新部署实例，现在仅用于升级Agent
func UpgradeAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	req.Namespace = model.AgentNamespace

	ch := opts.CrChan

	username, password, err := helmcommon.GetCharUsernameAndPassword(opts, cmd)
	if err != nil {
		return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}

	resp, err := opts.HelmClient.UpgradeRelease(&req, username, password)
	if err != nil {
		if req.ChartName == "choerodon-cluster-agent" && req.Namespace == model.AgentNamespace {
			go func() {
				//maybe avoid lot request devOps-service in a same time
				rand.Seed(time.Now().UnixNano())
				randWait := rand.Intn(20)
				time.Sleep(time.Duration(randWait) * time.Second)
				glog.Infof("start retry upgrade agent ...")
				ch.CommandChan <- cmd
			}()
		}
		return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	respB, err := json.Marshal(resp)
	if err != nil {
		return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.HelmReleaseUpgradeResourceInfo,
		Payload: string(respB),
	}
}

func ReSyncAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	fmt.Println("get command re_sync")
	opts.ControllerContext.ReSync()
	return nil, nil
}

// 检查命名空间是否存在，不存在则创建，添加helm标签同时调用helm升级函数，将helm实例从helm2升级到helm3
// 如果命名空间存在则检查labels，是否设置 "choerodon.io/helm-version":"helm3"
// 未设置helm标签，则添加helm标签并调用helm升级函数，将helm实例从helm2升级到helm3
func createNamespace(opts *commandutil.Opts, namespaceName string, releases []string) error {
	_, err := opts.KubeClient.GetKubeClient().CoreV1().Namespaces().Get(context.TODO(), namespaceName, metav1.GetOptions{})
	if err != nil {
		// 如果命名空间不存在的话，则创建
		if errors.IsNotFound(err) {
			_, err = opts.KubeClient.GetKubeClient().CoreV1().Namespaces().Create(context.TODO(), &corev1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name: namespaceName,
				},
			}, metav1.CreateOptions{})
			return err
		}
		return err
	}
	return nil
}

func getClusterInfo(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	listOpts := metav1.ListOptions{}

	serverVersion, err := opts.KubeClient.GetKubeClient().Discovery().ServerVersion()
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}

	namespaceList, err := opts.KubeClient.GetKubeClient().CoreV1().Namespaces().List(context.TODO(), listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}
	nodeList, err := opts.KubeClient.GetKubeClient().CoreV1().Nodes().List(context.TODO(), listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}
	podList, err := opts.KubeClient.GetKubeClient().CoreV1().Pods("").List(context.TODO(), listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}

	glog.Infof("current k8s version: %s", serverVersion.GitVersion)

	clusterInfo := ClusterInfo{
		Version:    serverVersion.GitVersion,
		Pods:       len(podList.Items),
		Namespaces: len(namespaceList.Items),
		Nodes:      len(nodeList.Items),
		ClusterId:  model.ClusterId,
	}
	response, err := json.Marshal(clusterInfo)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.ClusterGetInfo,
		Payload: string(response),
	}
}

func returnInitResult(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	if model.RestrictedModel {
		return nil, nil
	} else {
		return getClusterInfo(opts, cmd)
	}
}
