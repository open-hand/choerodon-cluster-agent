package agent

import (
	"encoding/json"
	"errors"
	"github.com/choerodon/choerodon-cluster-agent/pkg/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/golang/glog"
)

// todo reuse this code
func AddEnv(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var agentInitOpts model.AgentInitOptions
	err := json.Unmarshal([]byte(cmd.Payload), &agentInitOpts)

	if err != nil || len(agentInitOpts.Envs) < 1 {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvCreateFailed, err)
	}

	if err = opts.KubeClient.GetNamespace(agentInitOpts.Envs[0].Namespace); err == nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvCreateFailed, errors.New("env already exist"))
	}
	opts.Envs = append(opts.Envs, agentInitOpts.Envs[0])

	namespace := agentInitOpts.Envs[0].Namespace
	opts.Namespaces.Add(namespace)
	ns, err := createNamespace(opts.KubeClient, namespace)
	if ns == nil {
		glog.V(1).Infof("create namespace %s failed", namespace)
	}

	g := gitops.New(opts.Wg, opts.GitConfig, opts.GitRepos, opts.KubeClient, opts.Cluster, opts.CrChan)
	g.GitHost = agentInitOpts.GitHost

	if err := g.PrepareSSHKeys(opts.Envs, opts); err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	//启动控制器， todo: 后期移除
	opts.ControllerContext.ReSync()

	//启动repo、
	g.Envs = append(g.Envs, agentInitOpts.Envs[0])
	go g.WithStop(opts.StopCh)

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.InitAgentSucceed,
		Payload: cmd.Payload,
	}
}

func DeleteEnv(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var env model.EnvParas
	err := json.Unmarshal([]byte(cmd.Payload), &env)

	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvDelete, err)
	}

	controllerContext := opts.ControllerContext

	if !controllerContext.Namespaces.Contain(env.Namespace) {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvDelete, err)
	}

	controllerContext.Namespaces.Remove(env.Namespace)

	newEnvs := []model.EnvParas{}

	for index, envPara := range opts.Envs {

		if envPara.Namespace == env.Namespace {
			newEnvs = append(opts.Envs[0:index], opts.Envs[index+1:]...)
		}

	}
	opts.Envs = newEnvs

	if err := opts.HelmClient.DeleteNamespaceReleases(env.Namespace); err != nil {
		glog.V(1).Info(err)
	}
	if err := opts.KubeClient.DeleteNamespace(env.Namespace); err != nil {
		glog.V(1).Info(err)
	}

	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.EnvDeleteSucceed,
		Payload: cmd.Payload,
	}
}
