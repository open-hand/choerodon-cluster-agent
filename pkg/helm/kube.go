package helm

import (
	"fmt"

	"github.com/golang/glog"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/helm/pkg/helm/portforwarder"
	"k8s.io/helm/pkg/kube"
)

func setupConnection() error {
	if settings.TillerHost == "" {
		config, client, err := getKubeClient(settings.KubeContext)
		if err != nil {
			return err
		}

		tunnel, err := portforwarder.New(settings.TillerNamespace, client, config)
		if err != nil {
			return err
		}

		settings.TillerHost = fmt.Sprintf("127.0.0.1:%d", tunnel.Local)
		glog.V(1).Infof("Created tunnel using local port: '%d'", tunnel.Local)
	}

	// Set up the gRPC config.
	glog.V(1).Infof("SERVER: %q", settings.TillerHost)

	// Plugin support.
	return nil
}

func getKubeClient(context string) (*rest.Config, kubernetes.Interface, error) {
	config, err := configForContext(context)
	if err != nil {
		return nil, nil, err
	}
	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, fmt.Errorf("could not get Kubernetes client: %s", err)
	}
	return config, client, nil
}

func configForContext(context string) (*rest.Config, error) {
	config, err := kube.GetConfig(context, "").ClientConfig()
	if err != nil {
		return nil, fmt.Errorf("could not get Kubernetes config for context %q: %s", context, err)
	}
	return config, nil
}
