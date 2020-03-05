package options

import (
	"context"
	"flag"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	agentnamespace "github.com/choerodon/choerodon-cluster-agent/pkg/agent/namespace"
	agentsync "github.com/choerodon/choerodon-cluster-agent/pkg/agent/sync"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubectl"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	"github.com/choerodon/choerodon-cluster-agent/pkg/polaris/config"
	operatorutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/operator"
	"github.com/choerodon/choerodon-cluster-agent/pkg/version"
	"github.com/choerodon/choerodon-cluster-agent/pkg/websocket"
	"github.com/golang/glog"
	"github.com/operator-framework/operator-sdk/pkg/leader"
	"github.com/operator-framework/operator-sdk/pkg/log/zap"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	k8sclient "k8s.io/client-go/kubernetes"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"
	"net/http"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
	"strconv"
	"sync"
	"syscall"
	"time"
)

const (
	defaultGitSyncTag       = "agent-sync"
	defaultGitDevOpsSyncTag = "devops-sync"
	defaultGitNotesRef      = "choerodon"
)

var (
	metricsPort int32 = 8383
)

type AgentOptions struct {
	Listen       string
	UpstreamURL  string
	Token        string
	PrintVersion bool
	// kubernetes controller
	PlatformCode                  string
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
	ClusterId                     int32
	// git repo
	gitURL             string
	gitBranch          string
	gitPath            string
	gitUser            string
	gitEmail           string
	gitPollInterval    time.Duration
	gitTimeOut         time.Duration
	gitSyncTag         string
	gitDevOpsSyncTag   string
	gitNotesRef        string
	syncInterval       time.Duration
	kubernetesKubectl  string
	statusSyncInterval time.Duration
	syncAll            bool
	polarisFile        string
}

var log = logf.Log.WithName("cmd")

func printVersion() {
	log.Info(fmt.Sprintf("Go Version: %s", runtime.Version()))
	log.Info(fmt.Sprintf("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH))
}

func NewAgentCommand(f cmdutil.Factory) *cobra.Command {

	//pflag.CommandLine.AddFlagSet(zap.FlagSet())
	// Add flags registered by imported packages (e.g. glog and
	// controller-runtime)
	//pflag.CommandLine.AddGoFlagSet(flag.CommandLine)
	//pflag.Parse()
	logf.SetLogger(zap.Logger())

	options := NewAgentOptions()
	cmd := &cobra.Command{
		Use:  "choerodon-cluster-agent",
		Long: `Environment Agent`,
		Run: func(cmd *cobra.Command, args []string) {
			Run(options, f)
		},
	}
	// 给cmd绑定参数
	options.BindFlags(cmd.Flags())
	cmd.PersistentFlags().AddGoFlagSet(flag.CommandLine)

	return cmd
}

func NewAgentOptions() *AgentOptions {
	a := &AgentOptions{
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

func Run(o *AgentOptions, f cmdutil.Factory) {
	kube.ClusterId = o.ClusterId
	if o.PrintVersion {
		fmt.Println(version.GetVersion())
		os.Exit(0)
	}

	// init a channel to receive commands
	crChan := channel.NewCRChannel(100, 1000)

	errChan := make(chan error, 1)
	shutdown := make(chan struct{})
	shutdownWg := &sync.WaitGroup{}

	// graceful shutdown
	defer func() {
		glog.Errorf("exiting %s", <-errChan)
		close(shutdown)
		shutdownWg.Wait()
	}()

	// init helm env settings
	helm.InitEnvSettings()

	// --------------- operator sdk start  -----------------  //

	// Get a config to talk to the apiserver
	//cfg, err := config.GetConfig()
	//if err != nil {
	//	log.Error(err, "")
	//	os.Exit(1)
	//}

	ctx := context.TODO()

	// Become the leader before proceeding
	err := leader.Become(ctx, "c7n-agent-lock-"+strconv.Itoa(int(o.ClusterId)))
	if err != nil {
		log.Error(err, "")
		os.Exit(1)
	}

	cfg, _ := f.ToRESTConfig()

	// for controller-manager
	mgrs := &operatorutil.MgrList{}

	// new kubernetes clientf
	kubeClient, err := kube.NewClient(f)
	if err != nil {
		errChan <- err
		return
	}

	glog.Infof("Starting connect to tiller...")
	helmClient := helm.NewClient(kubeClient, cfg)
	glog.Infof("Tiller connect success")

	// todo: improve check k8s is working
	checkKube(kubeClient.GetKubeClient())

	glog.Infof("KubeClient init success.")

	// 需要listen de namespaces
	namespaces := agentnamespace.NewNamespaces()

	log.Info("Starting the Cmd.")
	// --------------- operator sdk end  -----------------  //

	// receive system int or term signal, send to err channel
	go func() {
		c := make(chan os.Signal)
		signal.Notify(c, syscall.SIGINT, syscall.SIGTERM)
		errChan <- fmt.Errorf("%s", <-c)
	}()

	appClient, err := websocket.NewClient(websocket.Token(o.Token), o.UpstreamURL, crChan, o.ClusterId)
	if err != nil {
		errChan <- err
		return
	}
	go appClient.Loop(shutdown, shutdownWg)

	//gitRemote := git.Remote{URL: o.gitURL}
	gitConfig := git.Config{
		Branch:          o.gitBranch,
		Path:            o.gitPath,
		UserName:        o.gitUser,
		GitUrl:          o.gitURL,
		UserEmail:       o.gitEmail,
		SyncTag:         o.gitSyncTag,
		DevOpsTag:       o.gitDevOpsSyncTag,
		NotesRef:        o.gitNotesRef,
		GitPollInterval: o.gitPollInterval,
	}

	agentctx := &agentsync.Context{
		Namespaces: namespaces,
		KubeClient: kubeClient,
		HelmClient: helmClient,
		CrChan:     crChan,
		StopCh:     shutdown,
	}

	cfg, err = f.ToRESTConfig()
	if err != nil {
		log.Error(err, "")
		os.Exit(127)
	}

	//ctx.StartControllers()
	var k8s *kubernetes.Cluster
	{
		kubectlPath := o.kubernetesKubectl
		if kubectlPath == "" {
			kubectlPath, err = exec.LookPath("kubectl")
		} else {
			_, err = os.Stat(kubectlPath)
		}
		if err != nil {
			glog.Fatal(err)
		}
		glog.Infof("kubectl %s", kubectlPath)
		cfg, _ := f.ToRESTConfig()
		kubectlApplier := kubectl.NewKubectl(kubectlPath, cfg)
		kubectlDescriber := kubectl.NewKubectl(kubectlPath, cfg)

		if err := kubectlApplier.ApplySingleObj("kube-system", model.CRD_YAML); err != nil {
			glog.V(1).Info(err)
		}

		k8s = kubernetes.NewCluster(kubeClient.GetKubeClient(), kubeClient.GetCrdClient(), mgrs, kubectlApplier, kubectlDescriber)
	}
	polarisConfig, err := config.ParseFile(o.polarisFile)
	if err != nil {
		glog.Error(err)
		os.Exit(0)
	}
	workerManager := agent.NewWorkerManager(
		mgrs,
		crChan,
		kubeClient,
		helmClient,
		appClient,
		k8s,
		&model.AgentInitOptions{},
		o.syncInterval,
		o.statusSyncInterval,
		o.gitTimeOut,
		gitConfig,
		agentctx,
		shutdownWg,
		shutdown,
		o.Token,
		o.PlatformCode,
		o.syncAll,
		&polarisConfig,
	)

	go workerManager.Start()
	shutdownWg.Add(1)

	go func() {
		errChan <- http.ListenAndServe(o.Listen, nil)
	}()

}

func (o *AgentOptions) BindFlags(fs *pflag.FlagSet) {
	fs.BoolVar(&o.PrintVersion, "version", false, "print the version number")
	fs.StringVar(&o.Listen, "listen", o.Listen, "address:port to listen on")
	fs.StringVar(&kube.AgentVersion, "agent-version", "", "agent version")
	// upstream
	fs.StringVar(&o.UpstreamURL, "connect", "", "Connect to an upstream service")
	fs.StringVar(&o.Token, "token", "", "Authentication token for upstream service")
	fs.Int32Var(&o.ClusterId, "clusterId", 0, "the env cluster id in devops")

	// kubernetes controller
	fs.StringVar(&o.PlatformCode, "choerodon-id", "", "choerodon platform id label")
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
	fs.DurationVar(&o.statusSyncInterval, "status-sync-interval", 3*time.Minute, "status sync interval")
	fs.DurationVar(&o.gitTimeOut, "git-timeout", 1*time.Minute, "git time out")
	fs.StringVar(&o.kubernetesKubectl, "kubernetes-kubectl", "", "Optional, explicit path to kubectl tool")
	fs.BoolVar(&o.syncAll, "sync-all", false, "sync all or change")
	fs.StringVar(&o.polarisFile, "polaris-file", "", "the polaris config file")
}

func checkKube(client *k8sclient.Clientset) {
	glog.Infof("check k8s role binding...")
	_, err := client.CoreV1().Pods("").List(meta_v1.ListOptions{})
	if err != nil {
		glog.Errorf("check role binding failed, %v", err)
		os.Exit(0)
	}
	glog.Infof(" k8s role binding succeed.")
}
