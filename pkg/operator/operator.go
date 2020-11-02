package operator

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/schemafunc"
	"github.com/choerodon/choerodon-cluster-agent/pkg/controller"
	controllerutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/controller"
	"github.com/operator-framework/operator-sdk/pkg/restmapper"
	"k8s.io/client-go/rest"
	crmanager "sigs.k8s.io/controller-runtime/pkg/manager"
	logf "sigs.k8s.io/controller-runtime/pkg/runtime/log"
)

var log = logf.Log.WithName("cmd")

var (
	metricsHost       = "0.0.0.0"
	metricsPort int32 = 8383
	healthHost        = "0.0.0.0"
	healthPort        = 8484
)

func NewMgr(cfg *rest.Config, namespace string) (crmanager.Manager, error) {
	// Create a new Cmd to provide shared dependencies and start components
	return crmanager.New(cfg, crmanager.Options{
		Namespace:              namespace,
		MapperProvider:         restmapper.NewDynamicRESTMapper,
		MetricsBindAddress:     "0",
		HealthProbeBindAddress: "0",
	})
}

func New(cfg *rest.Config, namespace string, args *controllerutil.Args) (crmanager.Manager, error) {

	mgr, err := NewMgr(cfg, namespace)
	if err != nil {
		return nil, err
	}
	log.Info("Registering Components.")

	// Setup Scheme for all resources
	for _, addSchemaFunc := range schemafunc.AddSchemaFuncs {
		if err := addSchemaFunc(mgr.GetScheme()); err != nil {
			return nil, err
		}
	}

	// Setup all Controllers
	if err := controller.AddToManager(mgr, args); err != nil {
		return nil, err
	}
	return mgr, nil
}
