package websocket

import (
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	pipeutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/pipe"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-cleanhttp"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
	util_url "github.com/choerodon/choerodon-cluster-agent/pkg/util/url"
)

const (
	// Time allowed to write a message to the peer.
	WriteWait      = 10 * time.Second
	initialBackOff = 1 * time.Second
	maxBackOff     = 60 * time.Second
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
)

var reconnectFlag = false

type Client interface {
	Loop(stopCh <-chan struct{}, done *sync.WaitGroup)
	PipeConnection(pipeID string, pipe pipeutil.Pipe) error
	PipeClose(pipeID string, pipe pipeutil.Pipe) error
	URL() *url.URL
}

type appClient struct {
	url            *url.URL
	token          Token
	crChannel      *channel.CRChan
	conn           *websocket.Conn
	quit           chan struct{}
	mtx            sync.Mutex
	client         *http.Client
	backgroundWait sync.WaitGroup
	pipeConns      map[string]*websocket.Conn
	respQueue      []*model.Packet
	clusterId      int32
}

func NewClient(
	t Token,
	endpoint string,
	crChannel *channel.CRChan,
	clusterId int32) (Client, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("no upstream URL given")
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint %s: %v", endpoint, err)
	}

	httpClient := cleanhttp.DefaultClient()

	c := &appClient{
		url:       endpointURL,
		token:     t,
		crChannel: crChannel,
		quit:      make(chan struct{}),
		client:    httpClient,
		pipeConns: make(map[string]*websocket.Conn),
		respQueue: make([]*model.Packet, 0, 100),
		clusterId: clusterId,
	}

	return c, nil
}

func (c *appClient) Loop(stop <-chan struct{}, done *sync.WaitGroup) {
	defer done.Done()

	glog.Info("Started websocket listening")

	backOff := 5 * time.Second
	errCh := make(chan error, 1)
	for {
		go func() {
			errCh <- c.connect()
		}()
		select {
		case err := <-errCh:
			if err != nil {
				glog.Error(err)
				reconnectFlag = true
			}
			time.Sleep(backOff)
		case <-stop:
			glog.Info("Shutting down agent")
			c.stop()
			return
		}
	}
}

func (c *appClient) connect() error {
	glog.V(1).Info("Start connect to DevOps service")
	var err error
	c.conn, err = dial(c.url.String(), c.token)
	if err != nil {
		return err
	}
	glog.V(1).Info("Connect to DevOps service success")

	// 建立连接，同步资源对象
	if reconnectFlag {
		c.crChannel.CommandChan <- newReConnectCommand()
	}

	defer func() {
		glog.V(1).Info("stop websocket connect")
		c.conn.Close()
	}()

	//ping-pong check
	if err := c.conn.SetReadDeadline(time.Now().Add(pongWait)); err != nil {
		return err
	}
	c.conn.SetPongHandler(func(string) error { c.conn.SetReadDeadline(time.Now().Add(pongWait)); return nil })
	ticker := time.NewTicker(pingPeriod)
	go func() {
		for {
			select {
			case <-ticker.C:
				c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					glog.Error(err)
					return
				}
			}
		}
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)

		c.conn.SetPingHandler(nil)
		for {
			var wp WsPacket
			err := c.conn.ReadJSON(&wp)
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
					glog.Error(err)
				}
				break
			}
			// for 平滑升级 we need to retain Payload field to handle upgrade agent command
			if wp.Data != nil {
				glog.V(1).Info("receive command: ", wp.Data)
				c.crChannel.CommandChan <- wp.Data
			} else {
				glog.V(1).Info("receive command: ", wp.Key, wp.Type)
				c.crChannel.CommandChan <- &model.Packet{
					Key:     wp.Key,
					Type:    wp.Type,
					Payload: wp.Payload,
				}
			}

		}
	}()

	end := 0
	for ; end < len(c.respQueue); end++ {
		c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
		resp := c.respQueue[end]
		if err := c.sendResponse(resp); err != nil {
			c.respQueue = c.respQueue[end:]
			return err
		}
	}
	c.respQueue = c.respQueue[end:]

	for {
		select {
		case <-done:
			return nil
		case resp, ok := <-c.crChannel.ResponseChan:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return nil
			}
			if err := c.sendResponse(resp); err != nil {
				glog.Error(err)
				c.respQueue = append(c.respQueue, resp)
			}
		}
	}
}

type WsPacket struct {
	Type string        `json:"type,omitempty"`
	Key  string        `json:"key,omitempty"`
	Data *model.Packet `json:"data"`
	// Deprecated; will remove at 0.20
	Payload string `json:"payload,omitempty"`
}

func (c *appClient) sendResponse(resp *model.Packet) error {

	wp := WsPacket{
		Type: "agent",
		Key:  fmt.Sprintf("cluster:%d", c.clusterId),
		Data: resp,
	}
	content, _ := json.Marshal(wp)
	glog.Infof("send response key %s, type %s", resp.Key, resp.Type)
	glog.V(1).Info("send response: ", string(content))
	return c.conn.WriteMessage(websocket.TextMessage, content)
}

func (c *appClient) hasQuit() bool {
	select {
	case <-c.quit:
		return true
	default:
		return false
	}
}

func (c *appClient) retainGoroutine() bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.hasQuit() {
		return false
	}
	c.backgroundWait.Add(1)
	return true
}

func (c *appClient) releaseGoroutine() {
	c.backgroundWait.Done()
}

func (c *appClient) stop() {
	c.mtx.Lock()
	close(c.quit)
	c.mtx.Unlock()

	c.backgroundWait.Wait()
}

func (c *appClient) URL() *url.URL {
	return c.url
}

func (c *appClient) PipeConnection(id string, pipe pipeutil.Pipe) error {
	go func() {
		glog.Infof("Pipe %s connection to %s starting", id, c.url)
		defer glog.Infof("Pipe %s connection to %s exiting", id, c.url)
		c.doWithBackOff(id, func() (bool, error) {
			return c.pipeConnection(id, pipe)
		})
	}()
	return nil
}

func (c *appClient) PipeClose(id string, pipe pipeutil.Pipe) error {
	//todo imp close func
	return nil
}

func (c *appClient) doWithBackOff(msg string, f func() (bool, error)) {
	if !c.retainGoroutine() {
		return
	}
	defer c.releaseGoroutine()

	backOff := initialBackOff
	for {
		done, err := f()
		if done {
			return
		}
		if err == nil {
			backOff = initialBackOff
			continue
		}
		glog.Errorf("Error doing %s for %s, backing off %s: %v", msg, c.url, backOff, err)
		select {
		case <-time.After(backOff):
		case <-c.quit:
			return
		}
		backOff *= 2
		if backOff > maxBackOff {
			backOff = maxBackOff
		}
	}
}

func (c *appClient) registerPipeConn(id string, conn *websocket.Conn) bool {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if c.hasQuit() {
		conn.Close()
		return false
	}
	c.pipeConns[id] = conn
	return true
}

func (c *appClient) closePipeConn(id string) {
	c.mtx.Lock()
	defer c.mtx.Unlock()
	if conn, ok := c.pipeConns[id]; ok {
		conn.Close()
		delete(c.pipeConns, id)
	}
}

func (c *appClient) pipeConnection(id string, pipe pipeutil.Pipe) (bool, error) {
	newURL, err := util_url.ParseURL(c.url, pipe.PipeType())
	if err != nil {
		return false, err
	}
	newURLStr := fmt.Sprintf("%s.%s:%s", newURL.String(), pipe.PipeType(), id)
	headers := http.Header{}
	conn, resp, err := dialWS(newURLStr, headers)
	if resp != nil && resp.StatusCode == http.StatusNotFound {
		glog.V(2).Info("response with not found")
		pipe.Close()
		return true, nil
	}
	if err != nil {
		return false, err
	}
	if !c.registerPipeConn(id, conn) {
		return true, nil
	}
	defer c.closePipeConn(id)

	_, remote := pipe.Ends()
	if err := pipe.CopyToWebsocket(remote, conn); err != nil {
		glog.Errorf("pipe copy to websocket: %v", err)
		if !IsExpectedWSCloseError(err) {
			return false, err
		}
	}
	pipe.Close()
	return true, nil
}

func newReConnectCommand() *model.Packet {
	return &model.Packet{
		Key:  "inter:inter",
		Type: model.ReSyncAgent,
	}
}
