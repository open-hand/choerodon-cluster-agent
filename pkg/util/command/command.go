package command

import (
	"github.com/choerodon/choerodon-cluster-agent/manager"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"time"
)

type Opts struct {
	GitTimeout time.Duration
	Namespaces *manager.Namespaces
	GitRepos   map[string]*git.Repo
}
