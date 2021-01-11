package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/command/helm"
)

func init() {
	Funcs.Add(model.CertManagerInstall, helm.InstallCertManager)
	Funcs.Add(model.CertManagerUninstall, helm.DeleteCertManagerRelease)

	Funcs.Add(model.HelmReleaseInstallResourceInfo, helm.InstallHelmRelease)
	Funcs.Add(model.HelmReleaseUpgradeResourceInfo, helm.UpgradeHelmRelease)

	// TODO devops 没有使用此功能，注释掉，等待以后有相关业务再处理
	//Funcs.Add(model.HelmReleaseRollback, helm.RollbackHelmRelease)

	Funcs.Add(model.HelmReleaseDelete, helm.DeleteHelmRelease)

	Funcs.Add(model.HelmInstallJobInfo, helm.InstallJobInfo)
	Funcs.Add(model.HelmUpgradeJobInfo, helm.UpgradeJobInfo)
	Funcs.Add(model.HelmCrossUpgradeJobInfo, helm.CrossUpgradeJobInfo)

	Funcs.Add(model.HelmReleaseStart, helm.StartHelmRelease)
	Funcs.Add(model.HelmReleaseStop, helm.StopHelmRelease)

	// TODO devops 没有使用此功能，注释掉，等待以后有相关业务再处理
	//Funcs.Add(model.HelmReleaseGetContent, helm.GetHelmReleaseContent)

	Funcs.Add(model.ResourceStatusSync, helm.SyncStatus)

	Funcs.Add(model.TestExecute, helm.ExecuteTestRelease)
	Funcs.Add(model.TestStatusRequest, helm.GetTestStatus)

	Funcs.Add(model.ChartMuseumAuthentication, helm.AddHelmAccount)
}
