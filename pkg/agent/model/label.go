package model

const (
	ParentWorkloadNameLabel = "choerodon.io/parent-workload-name"
	ParentWorkloadLabel     = "choerodon.io/parent-workload"
	WorkloadLabel           = "choerodon.io/workload"
	HelmVersion             = "choerodon.io/helm-version"
	MicroServiceConfig      = "choerodon.io/feature"
	ReleaseLabel            = "choerodon.io/release"
	NetworkLabel            = "choerodon.io/network"
	NetworkNoDelLabel       = "choerodon.io/no_delete"
	AgentVersionLabel       = "choerodon.io"
	CommitLabel             = "choerodon.io/commit"
	TlsSecretLabel          = "choerodon.io/tls-secret"
	C7NHelmReleaseClusterLabel      = "choerodon.io/C7NHelmRelease-cluster"
	C7NHelmReleaseOperateAnnotation = "choerodon.io/C7NHelmRelease-operate"
	TestLabel                       = "choerodon.io/test"
	EnvLabel                        = "choerodon.io/env"
	PvLabel                         = "choerodon.io/pv"
	NameLabel                       = "choerodon.io/name"
	PvcLabel                        = "choerodon.io/pvc"
	PvLabelValueFormat              = "pv-cluster-%s"
	PvcLabelValueFormat             = "pvc-cluster-%s"
)
