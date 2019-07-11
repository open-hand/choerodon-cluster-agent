package kubernetes

import (
	core_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	c7nv1alpha1 "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)

var (
	resourceKinds = make(map[string]resourceKind)
)

type resourceKind interface {
	getResources(c *Cluster, namespace string) ([]k8sResource, error)
}

func init() {
	resourceKinds["service"] = &serviceKind{}
	resourceKinds["ingress"] = &ingressKind{}
	resourceKinds["c7nhelmrelease"] = &c7nHelmReleaseKind{}
	resourceKinds["configmap"] = &configMap{}
	resourceKinds["secret"] = &secret{}
}

type k8sResource struct {
	k8sObject
	apiVersion string
	kind       string
	name       string
}

// ==============================================
// service
type serviceKind struct {
}

func (dk *serviceKind) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	services, err := c.client.Services(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range services.Items {
		_, noDelete := services.Items[i].Labels[model.NetworkNoDelLabel]
		if _, ok := services.Items[i].Labels[model.NetworkLabel]; !ok || noDelete {
			continue
		}
		k8sResources = append(k8sResources, makeServiceK8sResource(&services.Items[i]))
	}

	return k8sResources, nil
}
func makeServiceK8sResource(service *core_v1.Service) k8sResource {
	return k8sResource{
		apiVersion: "v1",
		kind:       "Service",
		name:       service.Name,
		k8sObject:  service,
	}
}

// ==============================================
// service
type ingressKind struct {
}

func (dk *ingressKind) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	ingresses, err := c.client.Ingresses(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range ingresses.Items {
		_, noDelete := ingresses.Items[i].Labels[model.NetworkNoDelLabel]
		if _, ok := ingresses.Items[i].Labels[model.NetworkLabel]; !ok || noDelete {
			continue
		}
		k8sResources = append(k8sResources, makeIngressK8sResource(&ingresses.Items[i]))
	}

	return k8sResources, nil
}

func makeIngressK8sResource(ingress *ext_v1beta1.Ingress) k8sResource {
	return k8sResource{
		apiVersion: "extensions/v1beta1",
		kind:       "Ingress",
		name:       ingress.Name,
		k8sObject:  ingress,
	}
}

// ==============================================
// choerodon.io/v1alpha1 C7NHelmRelease

type c7nHelmReleaseKind struct {
}

func (crk *c7nHelmReleaseKind) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	chrs, err := c.client.ChoerodonV1alpha1().C7NHelmReleases(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range chrs.Items {
		k8sResources = append(k8sResources, makeC7nHelmReleaseK8sResource(&chrs.Items[i]))
	}

	return k8sResources, nil
}

func makeC7nHelmReleaseK8sResource(chr *c7nv1alpha1.C7NHelmRelease) k8sResource {
	return k8sResource{
		apiVersion: "v1alpha1",
		kind:       "C7NHelmRelease",
		name:       chr.Name,
		k8sObject:  chr,
	}
}

// ==============================================
// configmap

type configMap struct {
}

func (cm *configMap) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	configMaps, err := c.client.ConfigMaps(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range configMaps.Items {
		cm := configMaps.Items[i]
		if cm.Labels[model.ReleaseLabel] == "" && cm.Labels[model.AgentVersionLabel] != "" {
			k8sResources = append(k8sResources, makeConfigMapK8sResource(&configMaps.Items[i]))
		}
	}

	return k8sResources, nil
}

func makeConfigMapK8sResource(cm *core_v1.ConfigMap) k8sResource {
	return k8sResource{
		apiVersion: "v1",
		kind:       "ConfigMap",
		name:       cm.Name,
		k8sObject:  cm,
	}
}

// ==============================================
// Secret

type secret struct {
}

func (s *secret) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	configMaps, err := c.client.Secrets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range configMaps.Items {
		cm := configMaps.Items[i]
		if cm.Labels[model.ReleaseLabel] == "" && cm.Labels[model.AgentVersionLabel] != "" {
			k8sResources = append(k8sResources, makeSecretK8sResource(&configMaps.Items[i]))
		}
	}

	return k8sResources, nil
}

func makeSecretK8sResource(cm *core_v1.Secret) k8sResource {
	return k8sResource{
		apiVersion: "v1",
		kind:       "Secret",
		name:       cm.Name,
		k8sObject:  cm,
	}
}
