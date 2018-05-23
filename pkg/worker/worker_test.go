package worker

import (
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/choerodon/choerodon-agent/pkg/model"
)

const (
	CmdFuncNormal   = "cmd_func_normal"
	CmdFuncNotExist = "cmd_func_not_exist"
	CmdFuncNewCmd   = "cmd_func_new_cmd"
)

func processCmdFuncNormal(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	return nil, &model.Response{
		Key:     cmd.Key,
		Type:    cmd.Type,
		Payload: "processCmdFuncNormal",
	}
}

func processCmdFuncNewCmd(w *workerManager, cmd *model.Command) ([]*model.Command, *model.Response) {
	var newCmds []*model.Command
	newCmd := &model.Command{
		Key:     cmd.Key,
		Type:    CmdFuncNormal,
		Payload: "processCmdFuncNormal",
	}
	newCmds = append(newCmds, newCmd)
	return newCmds, &model.Response{
		Key:     cmd.Key,
		Type:    cmd.Type,
		Payload: "processCmdFuncNewCmd",
	}
}

func workerTestSetupWorkerManager() (chan *model.Command, chan *model.Response) {
	commandChan := make(chan *model.Command)
	responseChan := make(chan *model.Response)

	registerCmdFunc(CmdFuncNormal, processCmdFuncNormal)
	registerCmdFunc(CmdFuncNewCmd, processCmdFuncNewCmd)

	wm := NewWorkerManager(commandChan, responseChan, nil, nil, nil, "")
	wm.Start()

	return commandChan, responseChan
}

func TestProcessCmdFuncNormal(t *testing.T) {
	cmdChan, respChan := workerTestSetupWorkerManager()
	cmd := &model.Command{
		Key:  "key",
		Type: CmdFuncNormal,
	}
	cmdChan <- cmd
	resp := <-respChan

	assert.Equal(t, CmdFuncNormal, resp.Type, "error response type")
	assert.Equal(t, "processCmdFuncNormal", resp.Payload, "error response payload")
}

func TestProcessCmdFuncNotExist(t *testing.T) {
	cmdChan, respChan := workerTestSetupWorkerManager()
	cmd := &model.Command{
		Key:  "key",
		Type: CmdFuncNotExist,
	}
	cmdChan <- cmd
	resp := <-respChan

	assert.Equal(t, CmdFuncNotExist, resp.Type, "error response type")
	assert.Equal(t, fmt.Sprintf("type %s not exist", cmd.Type), resp.Payload, "error response payload")
}

func TestProcessCmdFuncNewCmd(t *testing.T) {
	cmdChan, respChan := workerTestSetupWorkerManager()
	cmd := &model.Command{
		Key:  "key",
		Type: CmdFuncNewCmd,
	}
	cmdChan <- cmd
	resp1 := <-respChan
	resp2 := <-respChan

	assert.Equal(t, CmdFuncNewCmd, resp1.Type, "error response type")
	assert.Equal(t, "processCmdFuncNewCmd", resp1.Payload, "error response payload")
	assert.Equal(t, CmdFuncNormal, resp2.Type, "error response type")
	assert.Equal(t, "processCmdFuncNormal", resp2.Payload, "error response payload")
}

func BenchmarkProcessCmdFuncNormal(b *testing.B) {
	for i := 0; i < b.N; i++ {

	}
}
