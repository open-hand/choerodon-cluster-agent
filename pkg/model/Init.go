package model

import "time"

type GitInitConfig struct {
	SshKey string `json:"sshKey,omitempty"`
	GitUrl string `json:"gitUrl,omitempty"`
}

const CRD_YAML string = "apiVersion: apiextensions.k8s.io/v1beta1\n" +
	"kind: CustomResourceDefinition\n" +
	"metadata:\n" +
	"  name: c7nhelmreleases.choerodon.io\n" +
	"spec:\n" +
	"  group: choerodon.io\n" +
	"  names:\n" +
	"    kind: C7NHelmRelease\n" +
	"    listKind: C7NHelmReleaseList\n" +
	"    plural: c7nhelmreleases\n" +
	"    singular: c7nhelmrelease\n" +
	"  scope: Namespaced\n" +
	"  version: v1alpha1\n"

const CERT_MANAGER_CONFIG string = "apiVersion: certmanager.k8s.io/v1alpha1\n" +
	"kind: ClusterIssuer\n" +
	"metadata:\n" +
	"  name: letsencrypt-prod\n" +
	"spec:\n" +
	"  acme:\n" +
	"    server: https://acme-v02.api.letsencrypt.org/directory\n" +
	"    email: choerodon@vk.vu\n" +
	"    privateKeySecretRef:\n" +
	"      name: letsencrypt-prod\n" +
	"    http01: {}\n" +
	"---\n" +
	"apiVersion: certmanager.k8s.io/v1alpha1\n" +
	"kind: ClusterIssuer\n" +
	"metadata:\n" +
	"  name: localhost\n" +
	"spec:\n" +
	"  acme:\n" +
	"    server: https://acme-staging.api.letsencrypt.org/directory\n" +
	"    email: choerodon@vk.vu\n" +
	"    privateKeySecretRef:\n" +
	"      name: localhost\n" +
	"    http01: {}";

type AgentInitOptions struct {
	Envs    []EnvParas `json:"envs,omitempty"`
	GitHost string     `json:"gitHost,omitempty"`
}

type AgentStatus struct {
	EnvStatuses            []EnvStatus
	HelmStatus             string
	HelmOpDuration         time.Duration
	KubeStatus             string
	LastControllerSyncTime string
}

type EnvStatus struct {
	EnvCode       string
	EnvId         int32
	GitReady      bool
	GitOpDuration time.Duration
}

type EnvParas struct {
	Namespace string `json:"namespace,omitempty"`
	EnvId     int32  `json:"envId,omitempty"`
	GitRsaKey string `json:"gitRsaKey,omitempty"`
	GitUrl    string `json:"gitUrl,omitempty"`
}



type UpgradeInfo struct {
	Envs []OldEnv `json:"envs,omitempty"`
	Token string `json:"token,omitempty"`
	PlatformCode string `json:"platformCode,omitempty"`
}

type OldEnv struct {
	Namespace string `json:"namespace,omitempty"`
	EnvId     int    `json:"envId,omitempty"`
}

