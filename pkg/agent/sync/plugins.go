package sync

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/helm"
	"github.com/gin-gonic/gin/json"
)

func reportCertManager(ctx *Context) error {
	req := &helm.GetReleaseContentRequest{
		ReleaseName: "choerodon-cert-manager",
	}
	release, err := ctx.HelmClient.GetRelease(req)
	if err != nil {
		return nil
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
	syncFuncs = append(syncFuncs, reportCertManager)
}
