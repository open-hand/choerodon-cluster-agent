package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/helm"
)

func init() {
	Funcs.Add(model.CertManagerInstall, helm.CertManagerInstall)
	Funcs.Add(model.HelmReleaseRollback, helm.RollbackHelmRelease)
	Funcs.Add(model.HelmReleaseDelete, helm.DeleteHelmRelease)

	Funcs.Add(model.HelmInstallJobInfo, helm.InstallJobInfo)
	Funcs.Add(model.HelmUpgradeJobInfo, helm.UpgradeJobInfo)

	Funcs.Add(model.HelmReleaseStart, helm.StartHelmRelease)
	Funcs.Add(model.HelmReleaseStop, helm.StopHelmRelease)
	Funcs.Add(model.HelmReleaseGetContent, helm.GetHelmReleaseContent)
	Funcs.Add(model.ResourceStatusSync, helm.SyncStatus)

	Funcs.Add(model.TestExecute, helm.ExecuteTestRelease)
	Funcs.Add(model.TestStatusRequest, helm.GetTestStatus)
}
