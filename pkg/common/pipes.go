package common

import (
	"io"
	"sync"

	"github.com/gorilla/websocket"
)

type Pipe interface {
	CopyToWebsocket(io.ReadWriter, *websocket.Conn) error
	Ends() (io.ReadWriter, io.ReadWriter)

	Close() error
	Closed() bool
	OnClose(func())
	PipeType() string
}

const (
	Log  = "log"
	Exec = "exec"
)

type pipe struct {
	mtx      sync.Mutex
	wg       sync.WaitGroup
	local    io.ReadWriter
	remote   io.ReadWriter
	closers  []io.Closer
	closed   bool
	quit     chan struct{}
	onClose  func()
	pipeType string
}

func NewPipeFromEnds(local, remote io.ReadWriter, pipeType string) Pipe {
	return &pipe{
		local:    local,
		remote:   remote,
		quit:     make(chan struct{}),
		pipeType: pipeType,
	}
}

func NewPipe(pipeType string) Pipe {
	r1, w1 := io.Pipe()
	r2, w2 := io.Pipe()
	local := struct {
		io.Reader
		io.Writer
	}{
		r1, w2,
	}
	remote := struct {
		io.Reader
		io.Writer
	}{
		r2, w1,
	}
	return &pipe{
		local:  local,
		remote: remote,
		closers: []io.Closer{
			r1, r2, w1, w2,
		},
		quit:     make(chan struct{}),
		pipeType: pipeType,
	}
}

func (p *pipe) Close() error {
	p.mtx.Lock()
	var onClose func()
	if !p.closed {
		p.closed = true
		close(p.quit)
		for _, c := range p.closers {
			c.Close()
		}
		onClose = p.onClose
	}
	p.mtx.Unlock()
	p.wg.Wait()

	if onClose != nil {
		onClose()
	}
	return nil
}

func (p *pipe) Closed() bool {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	return p.closed
}

func (p *pipe) OnClose(f func()) {
	p.mtx.Lock()
	defer p.mtx.Unlock()
	p.onClose = f
}

func (p *pipe) Ends() (io.ReadWriter, io.ReadWriter) {
	return p.local, p.remote
}

func (p *pipe) CopyToWebsocket(end io.ReadWriter, conn *websocket.Conn) error {
	p.mtx.Lock()
	if p.closed {
		p.mtx.Unlock()
		return nil
	}
	p.wg.Add(1)
	p.mtx.Unlock()
	defer p.wg.Done()

	errors := make(chan error, 2)

	go func() {
		for {
			_, buf, err := conn.ReadMessage()
			if err != nil {
				errors <- err
				return
			}

			if p.Closed() {
				return
			}

			if _, err := end.Write(buf); err != nil {
				errors <- err
				return
			}
		}
	}()

	go func() {
		buf := make([]byte, 1024)
		for {
			n, err := end.Read(buf)
			if err != nil {
				errors <- err
				return
			}

			if p.Closed() {
				return
			}

			if err := conn.WriteMessage(websocket.BinaryMessage, buf[:n]); err != nil {
				errors <- err
				return
			}
		}
	}()

	select {
	case err := <-errors:
		return err
	case <-p.quit:
		return nil
	}
}

func (p *pipe) PipeType() string {
	return p.pipeType
}
