package sync

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
	"github.com/gin-gonic/gin/json"
)
//此方法在11.12日废除。
func reportCertManager(ctx *Context) error {
	req := &helm.GetReleaseContentRequest{
		ReleaseName: "choerodon-cert-manager",
	}
	release, err := ctx.HelmClient.GetRelease(req)
	if err != nil && !errors.IsNotFound(err) {
		return err
	}
	var payload []byte
	if release != nil {
		payload, _ = json.Marshal(release)
	}
	ctx.CrChan.ResponseChan <- &model.Packet{
		Key:     "none:none",
		Type:    model.CertManagerInfo,
		Payload: string(payload),
	}

	return nil
}

func init() {
	//初始化方法，废除。
	//syncFuncs = append(syncFuncs, reportCertManager)
}
