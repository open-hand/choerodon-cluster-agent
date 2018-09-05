package worker

import (
	"context"
	"time"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

const (
	// Timeout for git operations we're prepared to abandon
	gitOpTimeout = 15 * time.Minute
)

func init() {
	registerCmdFunc(model.GitOpsSync, gitOpsDoSync)
}

func gitOpsDoSync(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
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
