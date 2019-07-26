package command

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/command"
)

type Func func(w *command.Opts, cmd *model.Packet) ([]*model.Packet, *model.Packet)

var Funcs = FuncMap{}

type FuncMap map[string]Func

func (fs *FuncMap) Add(key string, f Func) {
	p := *fs
	p[key] = f
}
