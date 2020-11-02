package kubernetes

import (
	"context"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/certificate/apis/certmanager/v1alpha1"
	c7nv1alpha1 "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	core_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/extensions/v1beta1"
	meta_v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	client2 "sigs.k8s.io/controller-runtime/pkg/client"
)

var (
	ResourceKinds = make(map[string]ResourceKind)
)

type ResourceKind interface {
	GetResources(c *Cluster, namespace string) ([]K8sResource, error)
}

func init() {
	ResourceKinds["service"] = &serviceKind{}
	ResourceKinds["ingress"] = &ingressKind{}
	ResourceKinds["c7nhelmrelease"] = &c7nHelmReleaseKind{}
	ResourceKinds["configmap"] = &configMap{}
	ResourceKinds["secret"] = &secret{}
	ResourceKinds["persistentvolumeclaim"] = &persistentVolumeClaim{}
	ResourceKinds["persistentvolume"] = &persistentVolume{}
	ResourceKinds["certificate"] = &certificateKind{}
}

type K8sResource struct {
	K8sObject
	ApiVersion string
	Kind       string
	Name       string
}

// ==============================================
// certificate
type certificateKind struct {
}

func (dk *certificateKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	certificates, err := c.Client.Certificates(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range certificates.Items {
		K8sResources = append(K8sResources, makeCertificateK8sResource(&certificates.Items[i]))
	}

	return K8sResources, nil
}
func makeCertificateK8sResource(certificate *v1alpha1.Certificate) K8sResource {
	return K8sResource{
		ApiVersion: "v1alpha1",
		Kind:       "Certificate",
		Name:       certificate.Name,
		K8sObject:  certificate,
	}
}

// ==============================================
// service
type serviceKind struct {
}

func (dk *serviceKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	services, err := c.Client.Services(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range services.Items {
		_, noDelete := services.Items[i].Labels[model.NetworkNoDelLabel]
		if _, ok := services.Items[i].Labels[model.NetworkLabel]; !ok || noDelete {
			continue
		}
		K8sResources = append(K8sResources, makeServiceK8sResource(&services.Items[i]))
	}

	return K8sResources, nil
}
func makeServiceK8sResource(service *core_v1.Service) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "Service",
		Name:       service.Name,
		K8sObject:  service,
	}
}

// ==============================================
// service
type ingressKind struct {
}

func (dk *ingressKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	ingresses, err := c.Client.Ingresses(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range ingresses.Items {
		_, noDelete := ingresses.Items[i].Labels[model.NetworkNoDelLabel]
		if _, ok := ingresses.Items[i].Labels[model.NetworkLabel]; !ok || noDelete {
			continue
		}
		K8sResources = append(K8sResources, makeIngressK8sResource(&ingresses.Items[i]))
	}

	return K8sResources, nil
}

func makeIngressK8sResource(ingress *ext_v1beta1.Ingress) K8sResource {
	return K8sResource{
		ApiVersion: "extensions/v1beta1",
		Kind:       "Ingress",
		Name:       ingress.Name,
		K8sObject:  ingress,
	}
}

// ==============================================
// choerodon.io/v1alpha1 C7NHelmRelease

type c7nHelmReleaseKind struct {
}

func (crk *c7nHelmReleaseKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {

	client := c.Mgrs.Get(namespace).GetClient()

	instances := &c7nv1alpha1.C7NHelmReleaseList{}

	if err := client.List(context.TODO(), instances, &client2.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range instances.Items {
		release := instances.Items[i]
		if namespace == kube.AgentNamespace {
			if release.Labels[model.C7NHelmReleaseClusterLabel] == kube.ClusterId {
				K8sResources = append(K8sResources, makeC7nHelmReleaseK8sResource(&instances.Items[i]))
			}
		} else {
			K8sResources = append(K8sResources, makeC7nHelmReleaseK8sResource(&instances.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeC7nHelmReleaseK8sResource(chr *c7nv1alpha1.C7NHelmRelease) K8sResource {
	return K8sResource{
		ApiVersion: "choerodon.io/v1alpha1",
		Kind:       "C7NHelmRelease",
		Name:       chr.Name,
		K8sObject:  chr,
	}
}

// ==============================================
// configmap

type configMap struct {
}

func (cm *configMap) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	configMaps, err := c.Client.ConfigMaps(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range configMaps.Items {
		cm := configMaps.Items[i]
		if cm.Labels[model.ReleaseLabel] == "" && cm.Labels[model.AgentVersionLabel] != "" {
			K8sResources = append(K8sResources, makeConfigMapK8sResource(&configMaps.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeConfigMapK8sResource(cm *core_v1.ConfigMap) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "ConfigMap",
		Name:       cm.Name,
		K8sObject:  cm,
	}
}

// ==============================================
// Secret

type secret struct {
}

func (s *secret) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	configMaps, err := c.Client.Secrets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range configMaps.Items {
		cm := configMaps.Items[i]
		if cm.Labels[model.ReleaseLabel] == "" && cm.Labels[model.AgentVersionLabel] != "" {
			K8sResources = append(K8sResources, makeSecretK8sResource(&configMaps.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeSecretK8sResource(cm *core_v1.Secret) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "Secret",
		Name:       cm.Name,
		K8sObject:  cm,
	}
}

// ==============================================
// persistentVolumeClaim

type persistentVolumeClaim struct {
}

func (s *persistentVolumeClaim) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	persistentVolumeClaims, err := c.Client.PersistentVolumeClaims(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range persistentVolumeClaims.Items {
		pvc := persistentVolumeClaims.Items[i]
		if pvc.Labels[model.ReleaseLabel] == "" && pvc.Labels[model.AgentVersionLabel] != "" && pvc.Labels[model.PvcLabel] == fmt.Sprintf(model.PvcLabelValueFormat, kube.ClusterId) {
			K8sResources = append(K8sResources, makePersistentVolumeClaimK8sResource(&persistentVolumeClaims.Items[i]))
		}
	}

	return K8sResources, nil
}

func makePersistentVolumeClaimK8sResource(pvc *core_v1.PersistentVolumeClaim) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "PersistentVolumeClaim",
		Name:       pvc.Name,
		K8sObject:  pvc,
	}
}

// ==============================================
// persistentVolume

type persistentVolume struct {
}

func (s *persistentVolume) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	persistentVolumes, err := c.Client.PersistentVolumes().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range persistentVolumes.Items {
		pv := persistentVolumes.Items[i]
		if pv.Labels[model.ReleaseLabel] == "" && pv.Labels[model.AgentVersionLabel] != "" && pv.Labels[model.PvLabel] == fmt.Sprintf(model.PvLabelValueFormat, kube.ClusterId) && pv.Labels[model.EnvLabel] == namespace {
			K8sResources = append(K8sResources, makePersistentVolumeK8sResource(&persistentVolumes.Items[i]))
		}
	}

	return K8sResources, nil
}

func makePersistentVolumeK8sResource(pv *core_v1.PersistentVolume) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "PersistentVolume",
		Name:       pv.Name,
		K8sObject:  pv,
	}
}
