package worker

import (
	"context"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
)


func init() {
	registerCmdFunc(model.GitOpsSync, gitOpsDoSync)
}

func gitOpsDoSync(w *workerManager, cmd *model.Packet) ([]*model.Packet, *model.Packet) {
	ctx, cancel := context.WithTimeout(context.Background(), w.gitTimeout)
	err := w.gitRepos[cmd.Namespace()].Refresh(ctx)
	cancel()
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.GitOpsSyncFailed, err)
	}
	return nil, &model.Packet{
		Key:  cmd.Key,
		Type: model.GitOpsSync,
	}
}
