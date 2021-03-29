package model

const (
	//manager
	InitAgent    = "agent_init"
	AgentUpgrade = "agent_upgrade"
	ReSyncAgent  = "re_sync"
	// devOps-service暂无处理失败的逻辑
	InitAgentFailed  = "init_agent_failed"
	EnvCreateFailed  = "env_create_failed"
	EnvDeleteSucceed = "env_stop_succeed"
	EnvDelete        = "env_delete"
	EnvCreate        = "env_create"

	// Components
	CertManagerInstall   = "cert_manager_install"
	CertManagerStatus    = "cert_manager_status"
	CertManagerUninstall = "cert_manager_uninstall"
	//CertManagerUninstallStatus = "cert_manager_uninstall_status"
	// pod
	DeletePod = "delete_pod"

	// helm
	// HelmReleaseUpgrade 以前用于升级agent及重新部署实例，现仅用于升级agent
	// Deprecated: 将在0.20移除
	HelmReleaseUpgrade              = "helm_release_upgrade"
	HelmReleaseSyncedFailed         = "helm_release_sync_failed"
	HelmReleaseInstallFailed        = "helm_release_install_failed"
	HelmReleaseCrossUpgradeFailed   = "helm_release_cross_upgrade_failed"
	HelmInstallJobInfo              = "helm_install_job_info"
	HelmUpgradeJobInfo              = "helm_upgrade_job_info"
	HelmCrossUpgradeJobInfo         = "helm_cross_upgrade_job_info"
	HelmJobLog                      = "helm_job_log"
	HelmJobEvent                    = "helm_job_event"
	HelmReleaseRollback             = "helm_release_rollback"
	HelmReleaseRollbackFailed       = "helm_release_rollback_failed"
	HelmReleaseStart                = "helm_release_start"
	HelmReleaseStartFailed          = "helm_release_start_failed"
	HelmReleaseStop                 = "helm_release_stop"
	HelmReleaseStopFailed           = "helm_release_stop_failed"
	HelmReleaseDelete               = "helm_release_delete"
	HelmReleaseDeleteFailed         = "helm_release_delete_failed"
	HelmReleaseGetContent           = "helm_release_get_content"
	HelmReleaseGetContentFailed     = "helm_release_get_content_failed"
	ChartMuseumAuthentication       = "chart_museum_authentication"
	ChartMuseumAuthenticationFailed = "chart_museum_authentication_failed"

	HelmReleaseInstallResourceInfo = "helm_install_resource_info"
	HelmReleaseUpgradeResourceInfo = "helm_upgrade_resource_info"
	HelmPodEvent                   = "helm_pod_event"
	// automatic test
	TestExecute        = "test_execute"
	ExecuteTestSucceed = "execute_test_succeed"
	ExecuteTestFailed  = "execute_test_failed"
	TestJobLog         = "test_job_log"
	TestPodEvent       = "test_pod_event"
	TestPodUpdate      = "test_pod_update"
	TestStatusRequest  = "test_status"
	TestStatusResponse = "test_status_response"
	// network
	NetworkService             = "network_service"
	NetworkServiceFailed       = "network_service_failed"
	NetworkServiceUpdate       = "network_service_update"
	NetworkServiceDelete       = "network_service_delete"
	NetworkServiceDeleteFailed = "network_service_delete_failed"
	NetworkIngress             = "network_ingress"
	NetworkIngressFailed       = "network_ingress_failed"
	NetworkIngressDelete       = "network_ingress_delete"
	NetworkIngressDeleteFailed = "network_ingress_delete_failed"
	NetworkSync                = "network_sync"

	Cert_Issued = "cert_issued"
	Cert_Faild  = "cert_failed"
	// kubernetes resource
	ResourceUpdate = "resource_update"
	ConfigUpdate   = "config_update"
	NodeUpdate     = "node_update"
	NodeDelete     = "node_delete"

	ResourceDelete = "resource_delete"
	ResourceSync   = "resource_sync"
	NodeSync       = "node_sync"
	PodMetricsSync = "pod_metrics_sync"

	// kubernetes
	KubernetesGetLogs                   = "kubernetes_get_logs"
	KubernetesGetLogsFailed             = "kubernetes_get_logs_failed"
	KubernetesExec                      = "kubernetes_exec"
	KubernetesExecFailed                = "kubernetes_exec_failed"
	OperatePodCount                     = "operate_pod_count"
	OperatePodCountFailed               = "operate_pod_count_failed"
	OperatePodCountSuccess              = "operate_pod_count_succeed"
	OperateDockerRegistrySecret         = "operate_docker_registry_secret"
	OperateDockerRegistrySecretFailed   = "operate_docker_registry_secret_failed"
	DeletePersistentVolumeClaimByLabels = "delete_persistent_volume_claim_by_labels"

	// git ops
	GitOpsSync       = "git_ops_sync"
	GitOpsSyncFailed = "git_ops_sync_failed"
	GitOpsSyncEvent  = "git_ops_sync_event"

	ResourceStatusSyncEvent = "resource_status_sync_event"
	ResourceStatusSync      = "resource_status_sync"
	ResourceDescribe        = "resource_describe"
	Upgrade                 = "upgrade"

	CertManagerInfo = "cert_manager_info"

	// polaris
	PolarisRequest = "polaris_scan_cluster"

	// clusterInfo
	ClusterGetInfo       = "cluster_info"
	ClusterGetInfoFailed = "cluster_info_failed"
)

const (
	INSTALL       = "INSTALL"
	UPGRADE       = "UPGRADE"
	CROSS_UPGRADE = "CROSS-UPGRADE"
)
