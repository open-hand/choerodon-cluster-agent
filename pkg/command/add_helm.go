package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/helm"
)

func init() {
	Funcs.Add(model.HelmInstallRelease, helm.InstallHelmRelease)
	Funcs.Add(model.HelmReleaseUpgrade, helm.UpgradeHelmRelease)
	Funcs.Add(model.HelmReleaseRollback, helm.RollbackHelmRelease)
	Funcs.Add(model.HelmReleaseDelete, helm.DeleteHelmRelease)

	Funcs.Add(model.HelmReleasePreInstall, helm.PreInstallHelmRelease)
	Funcs.Add(model.HelmReleasePreUpgrade, helm.PreUpdateHelmRelease)

	Funcs.Add(model.HelmReleaseStart, helm.StartHelmRelease)
	Funcs.Add(model.HelmReleaseStop, helm.StopHelmRelease)
	Funcs.Add(model.HelmReleaseGetContent, helm.GetHelmReleaseContent)
	Funcs.Add(model.StatusSync, helm.SyncStatus)

	Funcs.Add(model.ExecuteTest, helm.ExecuteTestRelease)
	Funcs.Add(model.TestStatusRequest, helm.GetTestStatus)
}
