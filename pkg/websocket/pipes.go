package websocket

import (
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"io"
)

type pipe struct {
	pipeutil.Pipe
	id     string
	client Client
}

func newPipe(p pipeutil.Pipe, c Client, id string, key string) (pipeutil.Pipe, error) {
	pipe := &pipe{
		Pipe:   p,
		id:     id,
		client: c,
	}
	if err := c.PipeConnection(id, key, pipe); err != nil {
		return nil, err
	}
	return pipe, nil
}

var NewPipe = func(c Client, id string, pipeType string, key string) (pipeutil.Pipe, error) {
	return newPipe(pipeutil.NewPipe(pipeType), c, id, key)
}

func NewPipeFromEnds(local, remote io.ReadWriter, c Client, id string, pipeType string, key string) (pipeutil.Pipe, error) {
	return newPipe(pipeutil.NewPipeFromEnds(local, remote, pipeType), c, id, key)
}

func (p *pipe) Close() error {
	err1 := p.Pipe.Close()
	err2 := p.client.PipeClose(p.id, p)
	if err1 != nil {
		return err1
	}
	return err2
}
