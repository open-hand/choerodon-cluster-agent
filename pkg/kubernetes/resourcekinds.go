package kubernetes

import (
	"context"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/certificate/v1/apis/certmanager/v1"
	"github.com/choerodon/choerodon-cluster-agent/pkg/apis/certificate/v1alpha1/apis/certmanager/v1alpha1"
	c7nv1alpha1 "github.com/choerodon/choerodon-cluster-agent/pkg/apis/choerodon/v1alpha1"
	appsv1 "k8s.io/api/apps/v1"
	batch_v1 "k8s.io/api/batch/v1"
	batch_v1beata1 "k8s.io/api/batch/v1beta1"
	core_v1 "k8s.io/api/core/v1"
	ext_v1beta1 "k8s.io/api/networking/v1beta1"
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
	//ResourceKinds["persistentvolume"] = &persistentVolume{}
	ResourceKinds["certificate"] = &certificateKind{}
	ResourceKinds["Deployment"] = &deploymentKind{}
	ResourceKinds["DaemonSet"] = &daemonSetKind{}
	ResourceKinds["Job"] = &jobKind{}
	ResourceKinds["CronJob"] = &cronJobKind{}
	ResourceKinds["StatefulSet"] = &statefulSetKind{}
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
	if model.CertManagerVersion == "0.1.0" {
		// 获取v1alpha1的certificate资源
		v1Alpha1Certificates, err := c.Client.V1alpha1Certificates(namespace).List(meta_v1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range v1Alpha1Certificates.Items {
			K8sResources = append(K8sResources, makeV1alpha1CertificateK8sResource(&v1Alpha1Certificates.Items[i]))
		}
	}

	if model.CertManagerVersion == "1.1.1" {
		// 获取v1版本的certificate资源
		v1Certificates, err := c.Client.V1Certificates(namespace).List(context.Background(), meta_v1.ListOptions{})
		if err != nil {
			return nil, err
		}
		for i := range v1Certificates.Items {
			K8sResources = append(K8sResources, makeV1CertificateK8sResource(&v1Certificates.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeV1CertificateK8sResource(certificate *v1.Certificate) K8sResource {
	return K8sResource{
		ApiVersion: "v1",
		Kind:       "Certificate",
		Name:       certificate.Name,
		K8sObject:  certificate,
	}
}

func makeV1alpha1CertificateK8sResource(certificate *v1alpha1.Certificate) K8sResource {
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
		if namespace == model.AgentNamespace {
			if release.Labels[model.C7NHelmReleaseClusterLabel] == model.ClusterId {
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
	secrets, err := c.Client.Secrets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	var K8sResources []K8sResource
	for i := range secrets.Items {
		sc := secrets.Items[i]
		if sc.Labels[model.TlsSecretLabel] == "" && sc.Labels[model.ReleaseLabel] == "" && sc.Labels[model.AgentVersionLabel] != "" {
			K8sResources = append(K8sResources, makeSecretK8sResource(&secrets.Items[i]))
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
		if pvc.Labels[model.ReleaseLabel] == "" && pvc.Labels[model.AgentVersionLabel] != "" && pvc.Labels[model.PvcLabel] == fmt.Sprintf(model.PvcLabelValueFormat, model.ClusterId) {
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
	if model.RestrictedModel {
		return []K8sResource{}, nil
	}
	var K8sResources []K8sResource
	persistentVolumes, err := c.Client.PersistentVolumes().List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range persistentVolumes.Items {
		pv := persistentVolumes.Items[i]
		if pv.Labels[model.ReleaseLabel] == "" && pv.Labels[model.AgentVersionLabel] != "" && pv.Labels[model.PvLabel] == fmt.Sprintf(model.PvLabelValueFormat, model.ClusterId) && pv.Labels[model.EnvLabel] == namespace {
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

// ==============================================
// deployment
type deploymentKind struct {
}

func (s *deploymentKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	deploymentList, err := c.Client.Deployments(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range deploymentList.Items {
		deploy := deploymentList.Items[i]
		if deploy.Labels[model.WorkloadLabel] == "Deployment" {
			K8sResources = append(K8sResources, makeDeploymentK8sResource(&deploymentList.Items[i]))
		}
	}

	return K8sResources, nil
}
func makeDeploymentK8sResource(deploy *appsv1.Deployment) K8sResource {
	return K8sResource{
		ApiVersion: "apps/v1",
		Kind:       "Deployment",
		Name:       deploy.Name,
		K8sObject:  deploy,
	}
}

// ==============================================
// StatefulSet
type statefulSetKind struct {
}

func (s *statefulSetKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	statefulSetList, err := c.Client.StatefulSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range statefulSetList.Items {
		deploy := statefulSetList.Items[i]
		if deploy.Labels[model.WorkloadLabel] == "StatefulSet" {
			K8sResources = append(K8sResources, makeStatefulSetK8sResource(&statefulSetList.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeStatefulSetK8sResource(statefulSet *appsv1.StatefulSet) K8sResource {
	return K8sResource{
		ApiVersion: "apps/v1",
		Kind:       "StatefulSet",
		Name:       statefulSet.Name,
		K8sObject:  statefulSet,
	}
}

// ==============================================
// Job
type jobKind struct {
}

func (s *jobKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	jobList, err := c.Client.Jobs(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range jobList.Items {
		deploy := jobList.Items[i]
		if deploy.Labels[model.WorkloadLabel] == "Job" {
			K8sResources = append(K8sResources, makeJobK8sResource(&jobList.Items[i]))
		}
	}

	return K8sResources, nil
}
func makeJobK8sResource(job *batch_v1.Job) K8sResource {
	return K8sResource{
		ApiVersion: "batch/v1",
		Kind:       "Job",
		Name:       job.Name,
		K8sObject:  job,
	}
}

// ==============================================
// CronJob
type cronJobKind struct {
}

func (s *cronJobKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	cronJobList, err := c.Client.CronJobs(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range cronJobList.Items {
		deploy := cronJobList.Items[i]
		if deploy.Labels[model.WorkloadLabel] == "CronJob" {
			K8sResources = append(K8sResources, makeCronJobK8sResource(&cronJobList.Items[i]))
		}
	}

	return K8sResources, nil
}
func makeCronJobK8sResource(cronJob *batch_v1beata1.CronJob) K8sResource {
	return K8sResource{
		ApiVersion: "batch/v1beta1",
		Kind:       "CronJob",
		Name:       cronJob.Name,
		K8sObject:  cronJob,
	}
}

// ==============================================
// DaemonSet
type daemonSetKind struct {
}

func (s *daemonSetKind) GetResources(c *Cluster, namespace string) ([]K8sResource, error) {
	var K8sResources []K8sResource
	daemonSetList, err := c.Client.DaemonSets(namespace).List(meta_v1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for i := range daemonSetList.Items {
		deploy := daemonSetList.Items[i]
		if deploy.Labels[model.WorkloadLabel] == "DaemonSet" {
			K8sResources = append(K8sResources, makeDaemonSetK8sResource(&daemonSetList.Items[i]))
		}
	}

	return K8sResources, nil
}

func makeDaemonSetK8sResource(daemonset *appsv1.DaemonSet) K8sResource {
	return K8sResource{
		ApiVersion: "apps/v1",
		Kind:       "DaemonSet",
		Name:       daemonset.Name,
		K8sObject:  daemonset,
	}
}
