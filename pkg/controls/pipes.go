package controls

import (
	"io"

	"github.com/choerodon/choerodon-cluster-agent/ws"
	"github.com/choerodon/choerodon-cluster-agent/pkg/common"
)

type pipe struct {
	common.Pipe
	id     string
	client ws.WebSocketClient
}

func newPipe(p common.Pipe, c ws.WebSocketClient, id string) (common.Pipe, error) {
	pipe := &pipe{
		Pipe:   p,
		id:     id,
		client: c,
	}
	if err := c.PipeConnection(id, pipe); err != nil {
		return nil, err
	}
	return pipe, nil
}

var NewPipe = func(c ws.WebSocketClient, id string, pipeType string) (common.Pipe, error) {
	return newPipe(common.NewPipe(pipeType), c, id)
}

func NewPipeFromEnds(local, remote io.ReadWriter, c ws.WebSocketClient, id string, pipeType string) (common.Pipe, error) {
	return newPipe(common.NewPipeFromEnds(local, remote, pipeType), c, id)
}

func (p *pipe) Close() error {
	err1 := p.Pipe.Close()
	err2 := p.client.PipeClose(p.id, p)
	if err1 != nil {
		return err1
	}
	return err2
}
