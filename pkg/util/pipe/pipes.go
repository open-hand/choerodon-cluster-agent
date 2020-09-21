package pipe

import (
	"bufio"
	"io"
	"sync"
	"time"

	"github.com/gorilla/websocket"
)

type Pipe interface {
	CopyToWebsocketForLog(io.ReadWriter, *websocket.Conn) error
	CopyToWebsocketForExec(io.ReadWriter, *websocket.Conn) error
	Ends() (io.ReadWriter, io.ReadWriter)

	Close() error
	Closed() bool
	OnClose(func())
	PipeType() string
}

const (
	Log           = "agent_log"
	Exec          = "agent_exec"
	IntervalTime  = 3 * time.Second
	CloseWaitTime = 5 * time.Second
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

func (p *pipe) CopyToWebsocketForExec(end io.ReadWriter, conn *websocket.Conn) error {
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

func (p *pipe) CopyToWebsocketForLog(end io.ReadWriter, conn *websocket.Conn) error {
	p.mtx.Lock()
	if p.closed {
		p.mtx.Unlock()
		return nil
	}
	p.wg.Add(1)
	p.mtx.Unlock()
	defer func() {
		conn.Close()
		p.onClose()
	}()

	// 为了实现能够按行(也就是 '\n')读取
	r := bufio.NewReader(end)
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
		msgChan := make(chan []byte, 100)
		done := make(chan bool, 1)
		defer func() {
			done <- true
		}()
		go trafficAntiShake(nil, conn, p, done, msgChan, errors)
		for {
			buf, err := r.ReadBytes('\n')
			if err != nil {
				// 等待一段时间，让websocket有充足时间发送缓存中的消息
				time.Sleep(CloseWaitTime)
				errors <- err
				return
			}

			if p.Closed() {
				return
			}
			msgChan <- buf
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

// 流量防抖作用
// 在未使用该方法以前，有在较短时间内多次发送消息给客户端(这里指devops)，引起客户端线程数激增，性能下降
// 使用该方法后，在一定时间，会把读取的消息拼接起来，达到指定时间后再返回给客户端，减少了消息发送次数，客户端处理线程数随之减少
func trafficAntiShake(end io.ReadWriter, conn *websocket.Conn, p *pipe, done chan bool, msgChan chan []byte, errors chan error) {
	// 最大等待时间
	interval := time.NewTimer(500 * time.Millisecond)
	message := make([]byte, 0)
	message = append(message, <-msgChan...)
	for {
		select {
		// 如果done可读，退出协程
		case <-done:
			return
		// 达到最大发送等待时间，立即发送，并重置定时器
		case <-interval.C:
			if len(message) == 0 {
				// 即使没有消息发送，也需要重置最大发送等待定时器
				interval.Reset(IntervalTime)
				continue
			}
			if end != nil {
				if _, err := end.Write(message); err != nil {
					errors <- err
					return
				}
			}
			if conn != nil {
				if err := conn.WriteMessage(websocket.BinaryMessage, message); err != nil {
					errors <- err
					return
				}
			}
			message = []byte{}
			interval.Reset(IntervalTime)
		// 从websocket读到了消息，把消息写到message中，并重置定时器
		case msg := <-msgChan:
			message = append(message, msg...)
		}
	}
}
