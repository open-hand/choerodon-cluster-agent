package cmd

import (
	"fmt"
	"os"

	"github.com/golang/glog"
	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
	cmdutil "k8s.io/kubernetes/pkg/kubectl/cmd/util"

	"github.com/choerodon/choerodon-agent/pkg/appclient"
	"github.com/choerodon/choerodon-agent/pkg/helm"
	"github.com/choerodon/choerodon-agent/pkg/http"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	"github.com/choerodon/choerodon-agent/pkg/model"
	"github.com/choerodon/choerodon-agent/pkg/signals"
	"github.com/choerodon/choerodon-agent/pkg/version"
	"github.com/choerodon/choerodon-agent/pkg/worker"
	"k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/api/apps/v1beta1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"log"
	"k8s.io/client-go/tools/cache"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/watch"
	"time"
)

func NewAgentCommand(f cmdutil.Factory) *cobra.Command {
	options := NewAgentRunOptions()
	cmd := &cobra.Command{
		Use:  "choerodon-agent",
		Long: `Environment Agent`,
		Run: func(cmd *cobra.Command, args []string) {
			stopCh := signals.SetupSignalHandler()

			if err := options.Run(f, stopCh); err != nil {
				fmt.Fprintf(os.Stderr, "%v\n", err)
				os.Exit(1)
			}
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
	Namespace    string
	PrintVersion bool

	ConcurrentEndpointSyncs   int32
	ConcurrentServiceSyncs    int32
	ConcurrentRSSyncs         int32
	ConcurrentJobSyncs        int32
	ConcurrentDeploymentSyncs int32
	ConcurrentIngressSyncs    int32
	ConcurrentSecretSyncs     int32
	ConcurrentConfigMapSyncs  int32
	ConcurrentPodSyncs        int32
}

func NewAgentRunOptions() *AgentRunOptions {
	a := &AgentRunOptions{
		Listen:                    "0.0.0.0:8088",
		ConcurrentEndpointSyncs:   5,
		ConcurrentServiceSyncs:    1,
		ConcurrentRSSyncs:         1,
		ConcurrentJobSyncs:        3,
		ConcurrentDeploymentSyncs: 1,
		ConcurrentIngressSyncs:    1,
		ConcurrentSecretSyncs:     1,
		ConcurrentConfigMapSyncs:  1,
		ConcurrentPodSyncs:        1,
	}

	return a
}

func (o *AgentRunOptions) AddFlag(fs *pflag.FlagSet) {
	fs.StringVar(&o.Listen, "listen", o.Listen, "address:port to listen on")
	// Upstream
	fs.StringVar(&o.UpstreamURL, "connect", "", "Connect to an upstream service")
	fs.StringVar(&o.Token, "token", "", "Authentication token for upstream service")
	// kubernetes
	fs.StringVar(&o.Namespace, "namespace", "", "Kubernetes namespace")
	fs.BoolVar(&o.PrintVersion, "version", false, "print the version number")
	// Controller
	fs.Int32Var(&o.ConcurrentEndpointSyncs, "concurrent-endpoint-syncs", o.ConcurrentEndpointSyncs, "The number of endpoint syncing operations that will be done concurrently. Larger number = faster endpoint updating, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentServiceSyncs, "concurrent-service-syncs", o.ConcurrentServiceSyncs, "The number of services that are allowed to sync concurrently. Larger number = more responsive service management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentRSSyncs, "concurrent-replicaset-syncs", o.ConcurrentRSSyncs, "The number of replica sets that are allowed to sync concurrently. Larger number = more responsive replica management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentJobSyncs, "concurrent-job-syncs", o.ConcurrentJobSyncs, "The number of job that are allowed to sync concurrently. Larger number = more responsive replica management, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentDeploymentSyncs, "concurrent-deployment-syncs", o.ConcurrentDeploymentSyncs, "The number of deployment objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentIngressSyncs, "concurrent-ingress-syncs", o.ConcurrentIngressSyncs, "The number of ingress objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentSecretSyncs, "concurrent-secret-syncs", o.ConcurrentSecretSyncs, "The number of secret objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentConfigMapSyncs, "concurrent-configmap-syncs", o.ConcurrentConfigMapSyncs, "The number of config map objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
	fs.Int32Var(&o.ConcurrentPodSyncs, "concurrent-pod-syncs", o.ConcurrentPodSyncs, "The number of pod objects that are allowed to sync concurrently. Larger number = more responsive deployments, but more CPU (and network) load")
}

func (o *AgentRunOptions) Run(f cmdutil.Factory, stopCh <-chan struct{}) error {
	if o.PrintVersion {
		fmt.Println(version.GetVersion())
		os.Exit(0)
	}

	helm.InitEnvSettings()
	commandChan := make(chan *model.Command, 100)
	responseChan := make(chan *model.Response, 100)
	kubeClient, err := kube.NewClient(f)
	if err != nil {
		return err
	}
	helmClient := helm.NewClient(kubeClient, o.Namespace)
	kubeClientSet, err := f.KubernetesClientSet()
	if err != nil {
		glog.Fatalf("Error building kubernetes clientset: %s", err.Error())
	}
	_,err = kubeClientSet.CoreV1().Pods(o.Namespace).List(v1.ListOptions{})
	if err != nil {
		return err
	}




	appClient, err := appclient.NewClient(appclient.Token(o.Token), o.UpstreamURL, commandChan, responseChan)
	if err != nil {
		return err
	}
	defer appClient.Stop()

	workerManager := worker.NewWorkerManager(
		commandChan,
		responseChan,
		kubeClient,
		helmClient,
		appClient,
		o.Namespace,
	)
	httpServer := http.NewServer(o.Listen)
	ctx := CreateControllerContext(o, kubeClientSet, kubeClient, stopCh, responseChan)

	go workerManager.Start()
	go httpServer.Run()

	go initializer(kubeClientSet, ctx.Stop)

	StartControllers(ctx, NewControllerInitializers())
	ctx.InformerFactory.Start(ctx.Stop)

	return appClient.Start(stopCh)
}


func initializeDeployment(deployment *v1beta1.Deployment, clientset *kubernetes.Clientset) error {
	if deployment.ObjectMeta.GetInitializers() != nil {
		pendingInitializers := deployment.ObjectMeta.GetInitializers().Pending

		//
		//initializerName := ""
		if  pendingInitializers[0].Name != ""{
			glog.Infof("Initializing deployment: %s", deployment.Name)

			//o, err := runtime.NewScheme().DeepCopy(deployment)
			//if err != nil {
			//	return err
			//}
			//initializedDeployment := o.(*v1beta1.Deployment)
			initializedDeployment := deployment

			// Remove self from the list of pending Initializers while preserving ordering.
			if len(pendingInitializers) == 1 {
				initializedDeployment.ObjectMeta.Initializers = nil
			} else {
				initializedDeployment.ObjectMeta.Initializers.Pending = append(pendingInitializers[:0], pendingInitializers[1:]...)
			}
			initializedDeployment.GetObjectMeta().GetLabels()["init/label"] = "hello";
			_, err := clientset.AppsV1beta1().Deployments(deployment.Namespace).Update(initializedDeployment)
			if err != nil {
				return err;
			}
			return nil;

			//if requireAnnotation {
			//	a := deployment.ObjectMeta.GetAnnotations()
			//	_, ok := a[annotation]
			//	if !ok {
			//		log.Printf("Required '%s' annotation missing; skipping envoy container injection", annotation)
			//		_, err = clientset.AppsV1beta1().Deployments(deployment.Namespace).Update(initializedDeployment)
			//		if err != nil {
			//			return err
			//		}
			//		return nil
			//	}
			//}

			// Modify the Deployment's Pod template to include the Envoy container
			// and configuration volume. Then patch the original deployment.
			//initializedDeployment.Spec.Template.Spec.Containers = append(deployment.Spec.Template.Spec.Containers, c.Containers...)
			//initializedDeployment.Spec.Template.Spec.Volumes = append(deployment.Spec.Template.Spec.Volumes, c.Volumes...)
			//
			//oldData, err := json.Marshal(deployment)
			//if err != nil {
			//	return err
			//}
			//
			//newData, err := json.Marshal(initializedDeployment)
			//if err != nil {
			//	return err
			//}
			//
			//patchBytes, err := strategicpatch.CreateTwoWayMergePatch(oldData, newData, v1beta1.Deployment{})
			//if err != nil {
			//	return err
			//}
			//
			//_, err = clientset.AppsV1beta1().Deployments(deployment.Namespace).Patch(deployment.Name, types.StrategicMergePatchType, patchBytes)
			//if err != nil {
			//	return err
			//}
		}
	}

	return nil
}

func initializer(clientset *kubernetes.Clientset, stopCh <-chan struct{})  {


	// Watch uninitialized Deployments in all namespaces.
	restClient := clientset.AppsV1beta1().RESTClient()
	watchlist := cache.NewListWatchFromClient(restClient, "deployments", corev1.NamespaceAll, fields.Everything())

	// Wrap the returned watchlist to workaround the inability to include
	// the `IncludeUninitialized` list option when setting up watch clients.
	includeUninitializedWatchlist := &cache.ListWatch{
		ListFunc: func(options metav1.ListOptions) (runtime.Object, error) {
			options.IncludeUninitialized = true
			return watchlist.List(options)
		},
		WatchFunc: func(options metav1.ListOptions) (watch.Interface, error) {
			options.IncludeUninitialized = true
			return watchlist.Watch(options)
		},
	}

	resyncPeriod := 30 * time.Second

	_, controller := cache.NewInformer(includeUninitializedWatchlist, &v1beta1.Deployment{}, resyncPeriod,
		cache.ResourceEventHandlerFuncs{
			AddFunc: func(obj interface{}) {
				err := initializeDeployment(obj.(*v1beta1.Deployment), clientset)
				if err != nil {
					log.Println(err)
				}
			},
		},
	)
	controller.Run(stopCh)
}
