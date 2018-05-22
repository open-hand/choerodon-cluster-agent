package model

const (
	// helm
	HelmReleasePreInstall     = "helm_release_pre_install"
	HelmInstallRelease        = "helm_install_release"
	HelmReleaseInstallFailed  = "helm_release_install_failed"
	HelmReleasePreUpgrade     = "helm_release_pre_upgrade"
	HelmReleaseUpgrade        = "helm_release_upgrade"
	HelmReleaseUpgradeFailed  = "helm_release_upgrade_failed"
	HelmReleaseRollback       = "helm_release_rollback"
	HelmReleaseRollbackFailed = "helm_release_rollback_failed"
	HelmReleaseStart          = "helm_release_start"
	HelmReleaseStartFailed    = "helm_release_start_failed"
	HelmReleases              = "helm_releases"
	HelmReleaseStop           = "helm_release_stop"
	HelmReleaseStopFailed     = "helm_release_stop_failed"
	HelmReleaseDelete         = "helm_release_delete"
	HelmReleaseDeleteFailed   = "helm_release_delete_failed"
	HelmReleaseHookGetLogs    = "helm_release_hook_get_logs"
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
	// kubernetes resource
	ResourceUpdate = "resource_update"
	ResourceDelete = "resource_delete"
	// kubernetes
	KubernetesGetLogs       = "kubernetes_get_logs"
	KubernetesGetLogsFailed = "kubernetes_get_logs_failed"
	KubernetesExec          = "kubernetes_exec"
	KubernetesExecFailed    = "kubernetes_exec_failed"
)
