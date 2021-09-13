package model

const (
	ProjectId          = "choerodon.io/project_id"
	HelmVersion        = "choerodon.io/helm-version"
	MicroServiceConfig = "choerodon.io/feature"
	ReleaseLabel       = "choerodon.io/release"
	NetworkLabel       = "choerodon.io/network"
	NetworkNoDelLabel  = "choerodon.io/no_delete"
	AgentVersionLabel  = "choerodon.io"
	CommitLabel        = "choerodon.io/commit"
	TlsSecretLabel     = "choerodon.io/tls-secret"
	// 拼写错误，暂时不要更改
	CommandLabel                    = "choeroodn.io/command"
	AppServiceIdLabel               = "choerodon.io/app-service-id"
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
