package agent

import (
	"encoding/json"
	"errors"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/gitops"
	"github.com/choerodon/choerodon-cluster-agent/pkg/operator"
	commandutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/golang/glog"
	"os"
)

// todo reuse this code
func AddEnv(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var agentInitOpts model.AgentInitOptions
	err := json.Unmarshal([]byte(cmd.Payload), &agentInitOpts)

	if err != nil || len(agentInitOpts.Envs) < 1 {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvCreateFailed, err)
	}

	skipCheckNamespace := os.Getenv("SKIP_CHECK_EXIST_NAMESPACE") == "True"

	if err = opts.KubeClient.GetNamespace(agentInitOpts.Envs[0].Namespace); err == nil && agentInitOpts.Envs[0].Namespace != "choerodon" && !skipCheckNamespace {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvCreateFailed, errors.New("env already exist"))
	}
	opts.AgentInitOps.Envs = append(opts.AgentInitOps.Envs, agentInitOpts.Envs[0])

	namespace := agentInitOpts.Envs[0].Namespace
	opts.Namespaces.Add(namespace)
	if namespace != "choerodon" {
		ns, _ := createNamespace(opts.KubeClient, namespace)
		if ns == nil {
			glog.V(1).Infof("create namespace %s failed", namespace)
		}
	}

	g := gitops.New(opts.Wg, opts.GitConfig, opts.GitRepos, opts.KubeClient, opts.Cluster, opts.CrChan)
	g.GitHost = agentInitOpts.GitHost

	if err := g.PrepareSSHKeys(opts.AgentInitOps.Envs, opts); err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	//启动控制器， todo: 后期移除
	opts.ControllerContext.ReSync()

	cfg, err := opts.KubeClient.GetRESTConfig()
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	args := &controller.Args{
		CrChan:       opts.CrChan,
		HelmClient:   opts.HelmClient,
		KubeClient:   opts.KubeClient,
		PlatformCode: opts.PlatformCode,
		Namespaces:   opts.Namespaces,
	}
	mgr, err := operator.New(cfg, namespace, args)
	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
	}

	stopCh := make(chan struct{}, 1)

	// check success added avoid repeat watch
	if opts.Mgrs.AddStop(namespace, mgr, stopCh) {
		go func() {
			if err := mgr.Start(stopCh); err != nil {
				opts.CrChan.ResponseChan <- commandutil.NewResponseError(cmd.Key, model.InitAgentFailed, err)
			}
		}()
	}

	//启动repo、
	g.Envs = append(g.Envs, agentInitOpts.Envs[0])
	go g.WithStop(opts.StopCh)

	return nil, nil
}

func DeleteEnv(opts *commandutil.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	var env model.EnvParas
	err := json.Unmarshal([]byte(cmd.Payload), &env)

	if err != nil {
		return nil, commandutil.NewResponseError(cmd.Key, model.EnvDelete, err)
	}

	namespace := opts.Namespaces
	namespace.Remove(env.Namespace)

	newEnvs := make([]model.EnvParas, 0)

	for index, envPara := range opts.AgentInitOps.Envs {

		if envPara.Namespace == env.Namespace {
			newEnvs = append(opts.AgentInitOps.Envs[0:index], opts.AgentInitOps.Envs[index+1:]...)
		}
	}
	opts.Mgrs.Remove(env.Namespace)
	opts.AgentInitOps.Envs = newEnvs

	opts.GitRepos[env.Namespace].SyncChan <- struct{}{}
	opts.GitRepos[env.Namespace].RefreshChan <- struct{}{}
	delete(opts.GitRepos, env.Namespace)

	if err := opts.HelmClient.DeleteNamespaceReleases(env.Namespace); err != nil {
		glog.V(1).Info(err)
	}
	if skipCheckNamespace := os.Getenv("SKIP_CHECK_EXIST_NAMESPACE") == "True"; skipCheckNamespace {

	} else if err := opts.KubeClient.DeleteNamespace(env.Namespace); err != nil {
		glog.V(1).Info(err)
	}
	return nil, &model.Packet{
		Key:     cmd.Key,
		Type:    model.EnvDeleteSucceed,
		Payload: cmd.Payload,
	}
}
