package worker

import (
	"context"
	"github.com/choerodon/choerodon-agent/pkg/model"
)


func init() {
	registerCmdFunc(model.GitOpsSync, gitOpsDoSync)
}

func gitOpsDoSync(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	ctx, cancel := context.WithTimeout(context.Background(), w.gitTimeout)
	err := w.gitRepo.Refresh(ctx)
	cancel()
	if err != nil {
		return nil, NewResponseError(cmd.Key, model.GitOpsSyncFailed, err)
	}
	return nil, &model.Response{
		Key:  cmd.Key,
		Type: model.GitOpsSync,
	}
}
