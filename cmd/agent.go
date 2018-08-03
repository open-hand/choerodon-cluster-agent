package cmd

import (
	"fmt"
	"net/http"
	_ "net/http/pprof"
	"os"
	"os/exec"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/choerodon/choerodon-agent/pkg/appclient"
	chrclientset "github.com/choerodon/choerodon-agent/pkg/client/clientset/versioned"
	"github.com/choerodon/choerodon-agent/pkg/cluster"
	"github.com/choerodon/choerodon-agent/pkg/cluster/kubernetes"
	"github.com/choerodon/choerodon-agent/pkg/git"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
	"github.com/choerodon/choerodon-agent/pkg/version"
	"github.com/choerodon/choerodon-agent/pkg/worker"
	"github.com/choerodon/choerodon-agent/pkg/controller/c7nhelmrelease"
)

const (
	defaultGitSyncTag  = "choerodon-sync"
	defaultGitDevOpsSyncTag  = "GitOps"

	defaultGitNotesRef = "choerodon"
)

func NewAgentCommand(f cmdutil.Factory) *cobra.Command {
	options := NewAgentRunOptions()
	cmd := &cobra.Command{
		Use:  "choerodon-agent",
		Long: `Environment Agent`,
		Run: func(cmd *cobra.Command, args []string) {
			options.Run(f)
		},
	}
	options.AddFlag(cmd.Flags())
	f.BindFlags(cmd.PersistentFlags())
	f.BindExternalFlags(cmd.PersistentFlags())

	return cmd
}

type AgentRunOptions struct {
	Listen       string
	UpstreamURL  string
	Token        string
	PrintVersion bool
	// kubernetes controller
	Namespace                     string
	ConcurrentEndpointSyncs       int32
	ConcurrentServiceSyncs        int32
	ConcurrentRSSyncs             int32
	ConcurrentJobSyncs            int32
	ConcurrentDeploymentSyncs     int32
	ConcurrentIngressSyncs        int32
	ConcurrentSecretSyncs         int32
	ConcurrentConfigMapSyncs      int32
	ConcurrentPodSyncs            int32
	ConcurrentC7NHelmReleaseSyncs int32
	// git repo
	gitURL            string
	gitBranch         string
	gitPath           string
	gitUser           string
	gitEmail          string
	gitPollInterval   time.Duration
	gitSyncTag        string
	gitDevOpsSyncTag  string
	gitNotesRef       string
	syncInterval      time.Duration
	kubernetesKubectl string
}

func NewAgentRunOptions() *AgentRunOptions {
	a := &AgentRunOptions{
		Listen:                        "0.0.0.0:8088",
		ConcurrentEndpointSyncs:       5,
		ConcurrentServiceSyncs:        1,
		ConcurrentRSSyncs:             1,
		ConcurrentJobSyncs:            3,
		ConcurrentDeploymentSyncs:     1,
		ConcurrentIngressSyncs:        1,
		ConcurrentSecretSyncs:         1,
		ConcurrentConfigMapSyncs:      1,
		ConcurrentPodSyncs:            1,
		ConcurrentC7NHelmReleaseSyncs: 1,
	}

	return a
}

func (o *AgentRunOptions) AddFlag(fs *pflag.FlagSet) {
	fs.BoolVar(&o.PrintVersion, "version", false, "print the version number")
	fs.StringVar(&o.Listen, "listen", o.Listen, "address:port to listen on")
	// upstream
	fs.StringVar(&o.UpstreamURL, "connect", "", "Connect to an upstream service")
	fs.StringVar(&o.Token, "token", "", "Authentication token for upstream service")
	fs.Int32Var(&c7nhelmrelease.EnvId, "envId", 0, "the env agent id in devops")

	// kubernetes controller
	fs.StringVar(&o.Namespace, "namespace", "", "Kubernetes namespace")
	fs.Int32Var(&o.ConcurrentEndpointSyncs, "concurrent-endpoint-syncs", o.ConcurrentEndpointSyncs, "The number of endpoint syncing operations that will be done concurrently. Larger number = faster endpoint updating, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentServiceSyncs, "concurrent-service-syncs", o.ConcurrentServiceSyncs, "The number of services that are allowed to sync concurrently. Larger number = more responsive service management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentRSSyncs, "concurrent-replicaset-syncs", o.ConcurrentRSSyncs, "The number of replica sets that are allowed to sync concurrently. Larger number = more responsive replica management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentJobSyncs, "concurrent-job-syncs", o.ConcurrentJobSyncs, "The number of job that are allowed to sync concurrently. Larger number = more responsive replica management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentDeploymentSyncs, "concurrent-deployment-syncs", o.ConcurrentDeploymentSyncs, "The number of deployment objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentIngressSyncs, "concurrent-ingress-syncs", o.ConcurrentIngressSyncs, "The number of ingress objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentSecretSyncs, "concurrent-secret-syncs", o.ConcurrentSecretSyncs, "The number of secret objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentConfigMapSyncs, "concurrent-configmap-syncs", o.ConcurrentConfigMapSyncs, "The number of config map objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentPodSyncs, "concurrent-pod-syncs", o.ConcurrentPodSyncs, "The number of pod objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentC7NHelmReleaseSyncs, "concurrent-c7nhelmrelease-syncs", o.ConcurrentC7NHelmReleaseSyncs, "The number of c7nhelmrelease objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	// git repo
	fs.StringVar(&o.gitURL, "git-url", "", "URL of git repo manifests")
	fs.StringVar(&o.gitBranch, "git-branch", "master", "branch of git repo to use for manifests")
	fs.StringVar(&o.gitPath, "git-path", ".", "path within git repo to locate manifests (relative path)")
	fs.StringVar(&o.gitUser, "git-user", "Choerodon", "username to use as git committer")
	fs.StringVar(&o.gitEmail, "git-email", "support@choerodon.io", "email to use as git committer")
	fs.DurationVar(&o.gitPollInterval, "git-poll-interval", 5*time.Minute, "period at which to poll git repo for new commits")
	fs.StringVar(&o.gitSyncTag, "git-sync-tag", defaultGitSyncTag, "tag to use to mark sync progress for this cluster")
	fs.StringVar(&o.gitDevOpsSyncTag, "git-devops-sync-tag", defaultGitDevOpsSyncTag, "tag to use to mark sync progress for this cluster")
	fs.StringVar(&o.gitNotesRef, "git-notes-ref", defaultGitNotesRef, "ref to use for keeping commit annotations in git notes")
	fs.DurationVar(&o.syncInterval, "sync-interval", 5*time.Minute, "apply config in git to cluster at least this often, even if there are no new commits")
	fs.StringVar(&o.kubernetesKubectl, "kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
}

func (o *AgentRunOptions) Run(f cmdutil.Factory) {
	if o.PrintVersion {
		fmt.Println(version.GetVersion())
		os.Exit(0)
	}

	errChan := make(chan error)
	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}

	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errChan <- fmt.Errorf("%s", <-c)
	}()

	defer func() {
		glog.Errorf("exiting %s", <-errChan)
		close(shutdown)
		shutdownWg.Wait()
	}()

	helm.InitEnvSettings()
	commandChan := make(chan *model.Command, 100)
	responseChan := make(chan *model.Response, 100)
	kubeClient, err := kube.NewClient(f)
	if err != nil {
		errChan <- err
		return
	}
	helmClient := helm.NewClient(kubeClient, o.Namespace)
	kubeCfg, err := f.ClientConfig()
	if err != nil {
		glog.Fatalf("Error building kubeconfig: %s", err.Error())
	}
	kubeClientSet, err := f.KubernetesClientSet()
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	if err := checkKube(kubeClientSet, o.Namespace); err != nil {
		errChan <- err
		return
	}
	c7nClientset, err := chrclientset.NewForConfig(kubeCfg)
	if err != nil {
		glog.Fatalf("Error building choerodon clientset: %s", err.Error())
	}

	appClient, err := appclient.NewClient(appclient.Token(o.Token), o.UpstreamURL, commandChan, responseChan)
	if err != nil {
		errChan <- err
		return
	}

	gitRemote := git.Remote{URL: o.gitURL}
	gitConfig := git.Config{
		Branch:    o.gitBranch,
		Path:      o.gitPath,
		UserName:  o.gitUser,
		UserEmail: o.gitEmail,
		SyncTag:   o.gitSyncTag,
		DevOpsTag: o.gitDevOpsSyncTag,
		NotesRef:  o.gitNotesRef,
	}
	gitRepo := git.NewRepo(gitRemote, git.PollInterval(o.gitPollInterval))
	{
		shutdownWg.Add(1)
		go func() {
			err := gitRepo.Start(shutdown, shutdownWg)
			if err != nil {
				errChan <- err
			}
		}()
	}

	var k8sManifests cluster.Manifests
	var k8s cluster.Cluster
	{
		kubectl := o.kubernetesKubectl
		if kubectl == "" {
			kubectl, err = exec.LookPath("kubectl")
		} else {
			_, err = os.Stat(kubectl)
		}
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectl)
		kubectlApplier := kubernetes.NewKubectl(kubectl, kubeCfg)
		k8s = kubernetes.NewCluster(o.Namespace, kubeClientSet, c7nClientset, kubectlApplier)
		k8sManifests = &kubernetes.Manifests{Namespace: o.Namespace}
	}

	workerManager := worker.NewWorkerManager(
		commandChan,
		responseChan,
		kubeClient,
		helmClient,
		appClient,
		o.Namespace,
		gitConfig,
		gitRepo,
		o.syncInterval,
		k8sManifests,
		k8s,
	)

	ctx := CreateControllerContext(
		o,
		kubeClientSet,
		c7nClientset,
		kubeClient,
		helmClient,
		shutdown,
		commandChan,
		responseChan,
	)
	StartControllers(ctx, NewControllerInitializers())
	ctx.kubeInformer.Start(shutdown)
	ctx.c7nInformer.Start(shutdown)

	go workerManager.Start(shutdown, shutdownWg)
	shutdownWg.Add(1)
	go appClient.Loop(shutdown, shutdownWg)

	go func() {
		errChan <- http.ListenAndServe(o.Listen, nil)
	}()
}

func checkKube(client *k8sclient.Clientset, namespace string) error {
	_, err := client.CoreV1().Pods(namespace).List(meta_v1.ListOptions{})
	return err
}
