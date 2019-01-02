package model

const (

	//manager
	InitAgent        = "init_agent"
	ReSyncAgent      = "re_sync"
	InitAgentSucceed = "init_agent_succeed"
	InitAgentFailed  = "init_agent_failed"
	EnvCreateFailed  = "env_create_failed"
	EnvCreateSucceed = "env_create_succeed"
	EnvDeleteSucceed = "env_stop_succeed"
	EnvDeleteFailed  = "env_stop_failed"
	EnvDelete        = "delete_env"
	CreateEnv        = "create_env"
	NamespaceUpdate  = "namespace_update"

	// helm
	HelmReleaseSynced           = "helm_release_sync"
	HelmReleaseSyncedFailed     = "helm_release_sync_failed"
	HelmReleasePreInstall       = "helm_release_pre_install"
	HelmInstallRelease          = "helm_install_release"
	HelmReleaseInstallFailed    = "helm_release_install_failed"
	HelmReleasePreUpgrade       = "helm_release_pre_upgrade"
	HelmReleaseUpgrade          = "helm_release_upgrade"
	HelmReleaseUpgradeFailed    = "helm_release_upgrade_failed"
	HelmReleaseRollback         = "helm_release_rollback"
	HelmReleaseRollbackFailed   = "helm_release_rollback_failed"
	HelmReleaseStart            = "helm_release_start"
	HelmReleaseStartFailed      = "helm_release_start_failed"
	HelmReleases                = "helm_releases"
	HelmReleaseStop             = "helm_release_stop"
	HelmReleaseStopFailed       = "helm_release_stop_failed"
	HelmReleaseDelete           = "helm_release_delete"
	HelmReleaseDeleteFailed     = "helm_release_delete_failed"
	HelmReleaseHookGetLogs      = "helm_release_hook_get_logs"
	HelmReleaseGetContent       = "helm_release_get_content"
	HelmReleaseGetContentFailed = "helm_release_get_content_failed"
	// automatic test
	ExecuteTest        = "execute_test"
	ExecuteTestSucceed = "execute_test_succeed"
	ExecuteTestFailed  = "execute_test_failed"
	TestJobLog         = "test_job_log"
	TestPodEvent       = "test_pod_event"
	TestPodUpdate      = "test_pod_update"
	TestStatusRequest = "test_status"
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
	NodeUpdate = "node_update"
	NodeDelete = "node_delete"

	ResourceDelete = "resource_delete"
	ResourceSync   = "resource_sync"
	NodeSync   = "node_sync"

	//kubernetes event
	JobEvent = "job_event"

	ReleasePodEvent = "release_pod_event"

	// kubernetes
	KubernetesGetLogs       = "kubernetes_get_logs"
	KubernetesGetLogsFailed = "kubernetes_get_logs_failed"
	KubernetesExec          = "kubernetes_exec"
	KubernetesExecFailed    = "kubernetes_exec_failed"
	OperatePodCount         = "operate_pod_count"
	OperatePodCountFailed   = "operate_pod_count_failed"
	OperatePodCountSuccess   = "operate_pod_count_succeed"
	// git ops
	GitOpsSync       = "git_ops_sync"
	GitOpsSyncFailed = "git_ops_sync_failed"
	GitOpsSyncEvent  = "git_ops_sync_event"

	StatusSyncEvent      = "status_sync_event"
	StatusSync           = "status_sync"
	Upgrade              = "upgrade"
	UpgradeCluster       = "upgrade_cluster"
	UpgradeClusterFailed = "upgrade_cluster_failed"
	CertManagerInfo      = "cert_manager_info"
)
