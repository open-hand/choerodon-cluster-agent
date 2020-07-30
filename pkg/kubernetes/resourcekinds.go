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
	resourceKinds["persistentvolumeclaim"] = &persistentVolumeClaim{}
	resourceKinds["persistentvolume"] = &persistentVolume{}
	resourceKinds["certificate"] = &certificateKind{}
}

type k8sResource struct {
	k8sObject
	apiVersion string
	kind       string
	name       string
}

// ==============================================
// certificate
type certificateKind struct {
}

func (dk *certificateKind) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	var k8sResources []k8sResource
	certificates, err := c.client.Certificates(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range certificates.Items {
		k8sResources = append(k8sResources, makeCertificateK8sResource(&certificates.Items[i]))
	}

	return k8sResources, nil
}
func makeCertificateK8sResource(certificate *v1alpha1.Certificate) k8sResource {
	return k8sResource{
		apiVersion: "v1alpha1",
		kind:       "Certificate",
		name:       certificate.Name,
		k8sObject:  certificate,
	}
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

	client := c.mgrs.Get(namespace).GetClient()

	instances := &c7nv1alpha1.C7NHelmReleaseList{}

	if err := client.List(context.TODO(), instances, &client2.ListOptions{
		Namespace: namespace,
	}); err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range instances.Items {
		release := instances.Items[i]
		if namespace == kube.AgentNamespace {
			if release.Labels[model.C7NHelmReleaseClusterLabel] == kube.ClusterId {
				k8sResources = append(k8sResources, makeC7nHelmReleaseK8sResource(&instances.Items[i]))
			}
		} else {
			k8sResources = append(k8sResources, makeC7nHelmReleaseK8sResource(&instances.Items[i]))
		}
	}

	return k8sResources, nil
}

func makeC7nHelmReleaseK8sResource(chr *c7nv1alpha1.C7NHelmRelease) k8sResource {
	return k8sResource{
		apiVersion: "choerodon.io/v1alpha1",
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

// ==============================================
// persistentVolumeClaim

type persistentVolumeClaim struct {
}

func (s *persistentVolumeClaim) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	persistentVolumeClaims, err := c.client.PersistentVolumeClaims(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var k8sResources []k8sResource
	for i := range persistentVolumeClaims.Items {
		pvc := persistentVolumeClaims.Items[i]
		if pvc.Labels[model.ReleaseLabel] == "" && pvc.Labels[model.AgentVersionLabel] != "" && pvc.Labels[model.PvcLabel] == fmt.Sprintf(model.PvcLabelValueFormat, kube.ClusterId) {
			k8sResources = append(k8sResources, makePersistentVolumeClaimK8sResource(&persistentVolumeClaims.Items[i]))
		}
	}

	return k8sResources, nil
}

func makePersistentVolumeClaimK8sResource(pvc *core_v1.PersistentVolumeClaim) k8sResource {
	return k8sResource{
		apiVersion: "v1",
		kind:       "PersistentVolumeClaim",
		name:       pvc.Name,
		k8sObject:  pvc,
	}
}

// ==============================================
// persistentVolume

type persistentVolume struct {
}

func (s *persistentVolume) getResources(c *Cluster, namespace string) ([]k8sResource, error) {
	var k8sResources []k8sResource
	persistentVolumes, err := c.client.PersistentVolumes().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range persistentVolumes.Items {
		pv := persistentVolumes.Items[i]
		if pv.Labels[model.ReleaseLabel] == "" && pv.Labels[model.AgentVersionLabel] != "" && pv.Labels[model.PvLabel] == fmt.Sprintf(model.PvLabelValueFormat, kube.ClusterId) && pv.Labels[model.EnvLabel] == namespace {
			k8sResources = append(k8sResources, makePersistentVolumeK8sResource(&persistentVolumes.Items[i]))
		}
	}

	return k8sResources, nil
}

func makePersistentVolumeK8sResource(pv *core_v1.PersistentVolume) k8sResource {
	return k8sResource{
		apiVersion: "v1",
		kind:       "PersistentVolume",
		name:       pv.Name,
		k8sObject:  pv,
	}
}
