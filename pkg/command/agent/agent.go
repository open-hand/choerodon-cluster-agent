package agent

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm/upgrade"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/operator"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
	"github.com/golang/glog"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"math/rand"
	"strings"
	"time"
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
		err := createNamespace(opts, envPara.Namespace, envPara.Releases)
		if err != nil {
			return nil, commandutil.NewResponseError(cmd.Key, cmd.Type, err)
		}
	}
	namespaces.Set(nsList)

	//启动控制器， todo: 重启metrics
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

	testManagerNamespace := helm.TestNamespace
	nsList = append(nsList, testManagerNamespace)
	for _, ns := range nsList {
		if opts.Mgrs.IsExist(ns) {
			continue
		}
		mgr, err := operator.New(cfg, ns, args)
		if err != nil {
			return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
		}
		stopCh := make(chan struct{}, 1)
		// check success added avoid repeat watch
		if opts.Mgrs.AddStop(ns, mgr, stopCh) {
			go func() {
				if err := mgr.Start(stopCh); err != nil {
					opts.CrChan.ResponseChan <- commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
				}
			}()
		}
	}

	g.Envs = agentInitOpts.Envs
	opts.AgentInitOps.Envs = agentInitOpts.Envs
	go g.WithStop(opts.StopCh)

	listOpts := metav1.ListOptions{}

	serverVersion, err := opts.KubeClient.GetKubeClient().Discovery().ServerVersion()
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}

	namespaceList, err := opts.KubeClient.GetKubeClient().CoreV1().Namespaces().List(listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}
	nodeList, err := opts.KubeClient.GetKubeClient().CoreV1().Nodes().List(listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}
	podList, err := opts.KubeClient.GetKubeClient().CoreV1().Pods("").List(listOpts)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.ClusterGetInfoFailed, err)
	}

	clusterInfo := ClusterInfo{
		Version:    serverVersion.Major + "." + serverVersion.Minor,
		Pods:       len(podList.Items),
		Namespaces: len(namespaceList.Items),
		Nodes:      len(nodeList.Items),
		ClusterId:  int(kube.ClusterId),
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

// 以前用于重新部署实例，现在仅用于升级Agent
func UpgradeAgent(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var req helm.UpgradeReleaseRequest
	err := json.Unmarshal([]byte(cmd.Payload), &req)
	if err != nil {
		return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
	}
	if req.Namespace == "" {
		req.Namespace = cmd.Namespace()
	}

	ch := opts.CrChan

	// 获取agent的deployment的标签helm的值是否为helm3，
	// 不是helm3，先getRelease，查看helm3是否管理该agent
	// release不为nil，表示helm3管理该agent，更新标签
	// release为nil，表示helm2管理该agent，从helm2版本升级到helm3版本，然后更新标签
	if req.ChartName == "choerodon-cluster-agent" && req.Namespace == "choerodon" {

		// 先判断标签的值
		deployment, err := opts.KubeClient.GetKubeClient().ExtensionsV1beta1().Deployments(req.Namespace).Get(req.ReleaseName, metav1.GetOptions{})
		if err != nil {
			return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
		}
		labels := deployment.ObjectMeta.GetLabels()

		// 再判断agent实例是否由helm3进行管理的
		if labels["helm"] != "helm3" {
			releaseRequest := &helm.GetReleaseContentRequest{
				ReleaseName: req.ReleaseName,
				Namespace:   req.Namespace,
			}
			rls, _ := opts.HelmClient.GetRelease(releaseRequest)

			// 实例由helm3管理，更新标签
			if rls != nil {
				labels["helm"] = "helm3"
				deployment.SetLabels(labels)
				opts.KubeClient.GetKubeClient().ExtensionsV1beta1().Deployments(req.Namespace).Update(deployment)
			} else {
				// 实例由helm2管理，先升级成helm3管理，然后更新标签
				err = upgrade.RunConvert(req.ReleaseName)
				// 如果从helm2升级到helm3没有问题，就清理helm2的数据并给agent的deployment添加标签
				if err == nil {
					err = upgrade.RunCleanup(req.ReleaseName)
					labels["helm"] = "helm3"
					deployment.SetLabels(labels)
					opts.KubeClient.GetKubeClient().ExtensionsV1beta1().Deployments(req.Namespace).Update(deployment)
				} else {
					return nil, commandutil.NewResponseErrorWithCommit(cmd.Key, req.Commit, model.HelmReleaseInstallFailed, err)
				}
			}
		}
	}

	resp, err := opts.HelmClient.UpgradeRelease(&req)
	if err != nil {
		if req.ChartName == "choerodon-cluster-agent" && req.Namespace == "choerodon" {
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
// 如果命名空间存在则检查labels，是否设置 "helm":"helm3"
// 未设置helm标签，则添加helm标签并调用helm升级函数，将helm实例从helm2升级到helm3
func createNamespace(opts *commandutil.Opts, namespaceName string, releases []string) error {
	ns, err := opts.KubeClient.GetKubeClient().CoreV1().Namespaces().Get(namespaceName, metav1.GetOptions{})
	if err != nil {
		// 如果命名空间不存在的话，则创建
		if errors.IsNotFound(err) {
			_, err = opts.KubeClient.GetKubeClient().CoreV1().Namespaces().Create(&v1.Namespace{
				ObjectMeta: metav1.ObjectMeta{
					Name:   namespaceName,
					Labels: map[string]string{"helm": "helm3"},
				},
			})
			return err
		}
		return err
	}

	labels := ns.Labels
	// 如果命名空间存在，则检查labels标签
	if _, ok := labels["helm"]; !ok {
		return update(opts, releases, namespaceName, labels)
	}
	return nil
}

func update(opts *commandutil.Opts, releases []string, namespaceName string, labels map[string]string) error {
	releaseCount := len(releases)
	if releaseCount != 0 {
		for i := 0; i < releaseCount; i++ {
			getReleaseRequest := &helm.GetReleaseContentRequest{
				ReleaseName: releases[i],
				Namespace:   namespaceName,
			}

			// 查看该实例是否已经升级到helm3
			_, err := opts.HelmClient.GetRelease(getReleaseRequest)
			if err != nil {
				if strings.Contains(err.Error(), helm.ErrReleaseNotFound) {
					err = upgrade.RunConvert(releases[i])
					if err != nil {
						return err
					}
				} else {
					return err
				}
			}
		}

		upgradedReleases, err := opts.HelmClient.ListRelease(namespaceName)
		if err != nil {
			return err
		}
		if len(upgradedReleases) != releaseCount {
			return fmt.Errorf("env %s : failed to upgrade helm2 to helm3 ", namespaceName)
		}

		// 将每个实例的helm2版本信息移除
		for i := 0; i < releaseCount; i++ {
			upgrade.RunCleanup(releases[i])
		}
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	labels["helm"] = "helm3"
	_, err := opts.KubeClient.GetKubeClient().CoreV1().Namespaces().Update(&v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name:   namespaceName,
			Labels: map[string]string{"helm": "helm3"},
		},
	})
	return err
}
