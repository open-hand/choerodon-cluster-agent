package model

import "time"

var CRD_YAMLS = make(map[string]string)

const (
	V1       = "v1"
	V1_BETA1 = "v1beta1"
)

func init() {
	CrdC7NHemReleaseYamlV1Beta1 := "apiVersion: apiextensions.k8s.io/v1beta1\n" +
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
	CRD_YAMLS[V1_BETA1] = CrdC7NHemReleaseYamlV1Beta1

	CrdC7NHemReleaseYamlV1 := "apiVersion: apiextensions.k8s.io/v1\n" +
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
		"  versions: \n" +
		"    - name: v1alpha1 \n" +
		"      storage: true \n" +
		"      served: true \n" +
		"      schema:\n" +
		"        openAPIV3Schema:\n" +
		"          type: object\n" +
		"          properties:\n" +
		"            spec:\n" +
		"              type: object\n" +
		"              properties:\n" +
		"                chartName:\n" +
		"                  type: string\n" +
		"                chartVersion:\n" +
		"                  type: string\n" +
		"                commandId:\n" +
		"                  type: integer\n" +
		"                imagePullSecrets:\n" +
		"                  type: array\n" +
		"                  items: \n" +
		"                    type: object\n" +
		"                    properties:\n" +
		"                      name:\n" +
		"                        type: string\n" +
		"                repoUrl:\n" +
		"                  type: string\n" +
		"                source:\n" +
		"                  type: string\n" +
		"                v1AppServiceId:\n" +
		"                  type: string\n" +
		"                v1CommandId:\n" +
		"                  type: string\n" +
		"                appServiceId:\n" +
		"                  type: integer\n" +
		"                values:\n" +
		"                  type: string"
	CRD_YAMLS[V1] = CrdC7NHemReleaseYamlV1
}

const CertManagerClusterIssuer = `apiVersion: cert-manager.io/v1
kind: ClusterIssuer
metadata:
  name: letsencrypt-prod
spec:
  acme:
    server: https://acme-v02.api.letsencrypt.org/directory
    email: {{ .ACME_EMAIL }} 
    privateKeySecretRef:
      name: letsencrypt-prod
    solvers:
      - http01:
          ingress:
            class: nginx`

type GitInitConfig struct {
	SshKey string `json:"sshKey,omitempty"`
	GitUrl string `json:"gitUrl,omitempty"`
}

type AgentInitOptions struct {
	Envs                    []EnvParas `json:"envs,omitempty"`
	GitHost                 string     `json:"gitHost,omitempty"`
	AgentName               string     `json:"agentName,omitempty"`
	CertManagerVersion      string     `json:"certManagerVersion,omitempty"`
	RepoConcurrencySyncSize int        `json:"repoConcurrencySyncSize,omitempty"`
}

type CertManagerStatusInfo struct {
	Status       string `json:"status,omitempty"`
	ChartVersion string `json:"chartVersion,omitempty"`
	Namespace    string `json:"namespace,omitempty"`
	ReleaseName  string `json:"releaseName,omitempty"`
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
	EnvId         int64
	GitReady      bool
	GitOpDuration time.Duration
}

type EnvParas struct {
	Namespace string   `json:"namespace,omitempty"`
	EnvId     int64    `json:"envId,omitempty"`
	GitRsaKey string   `json:"gitRsaKey,omitempty"`
	GitUrl    string   `json:"gitUrl,omitempty"`
	Releases  []string `json:"instances,omitempty"`
}

type UpgradeInfo struct {
	Envs         []OldEnv `json:"envs,omitempty"`
	Token        string   `json:"token,omitempty"`
	PlatformCode string   `json:"platformCode,omitempty"`
}

type OldEnv struct {
	Namespace string `json:"namespace,omitempty"`
	EnvId     int64  `json:"envId,omitempty"`
}
