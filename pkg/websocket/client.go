package websocket

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/channel"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
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
	// Time allowed to read the next pong message from the peer.
	pongWait = 60 * time.Second
	// Send pings to peer with this period. Must be less than pongWait.
	pingPeriod = (pongWait * 9) / 10
	// Maximum length that message can be sent
	maxLength = 4194304
)

var ReconnectFlag = false
var GitStopChan = make(chan struct{})
var GitStopped = false

type Client interface {
	Loop(stopCh <-chan struct{}, done *sync.WaitGroup)
	PipeConnection(pipeID string, key string, token string, pipe pipeutil.Pipe) error
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
	clusterId      string
}

// Token 参数里面的token
// endPoint devops的websocket地址
func NewClient(
	t Token,
	endpoint string,
	crChannel *channel.CRChan,
	clusterId string) (Client, error) {
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
				ReconnectFlag = true
				if !GitStopped {
					close(GitStopChan)
					GitStopped = true
				}
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
	ctx, cancel := context.WithCancel(context.Background())
	c.conn, err = dial(c.url.String(), c.token)
	if err != nil {
		return err
	}
	glog.V(1).Info("Connect to DevOps service success")

	// 建立连接，同步资源对象
	if ReconnectFlag {
		c.crChannel.CommandChan <- newReConnectCommand()
		// 重置该标志
		ReconnectFlag = false
		GitStopChan = make(chan struct{})
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
	go func(ctx context.Context) {
		for {
			select {
			case <-ticker.C:
				c.mtx.Lock()
				c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
				if err := c.conn.WriteMessage(websocket.PingMessage, nil); err != nil {
					glog.Error(err)
					c.mtx.Unlock()
					return
				}
				c.mtx.Unlock()
			case <-ctx.Done():
				return
			}
		}
	}(ctx)

	go func(cancel context.CancelFunc) {
		defer cancel()

		c.conn.SetPingHandler(nil)
		for {
			var wp WsReceivePacket
			err := c.conn.ReadJSON(&wp)
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
					glog.Error(err)
				}
				break
			}
			packet := &model.Packet{}
			glog.V(1).Infof("receive message:\n>>>\nKey: %s\nType: %s\nMessage: %s\nGroup %s\n<<<", wp.Key, wp.Type, wp.Message, wp.Group)
			err = json.Unmarshal([]byte(wp.Message), packet)
			if err != nil {
				glog.Error(err)
			}
			c.crChannel.CommandChan <- packet
		}
	}(cancel)

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
		case <-ctx.Done():
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

type WsReceivePacket struct {
	Type    string `json:"type,omitempty"`
	Key     string `json:"key,omitempty"`
	Message string `json:"message"`
	Group   string `json:"group,omitempty"`
}

func (c *appClient) sendResponse(resp *model.Packet) error {

	content, _ := json.Marshal(resp)
	if len(content) >= maxLength {
		glog.Infof("the length of message key %s, type %s is more than %d , discard", resp.Key, resp.Type, maxLength)
		return nil
	}
	glog.Infof("send response key %s, type %s", resp.Key, resp.Type)
	glog.V(1).Info("send response: ", string(content))
	c.mtx.Lock()
	defer c.mtx.Unlock()
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

func (c *appClient) PipeConnection(id string, key string, token string, pipe pipeutil.Pipe) error {
	go func() {
		glog.Infof("Pipe %s connection to %s starting", id, c.url)
		defer glog.Infof("Pipe %s connection to %s closed", id, c.url)
		c.doWithBackOff(id, func() (bool, error) {
			return c.pipeConnection(id, key, token, pipe)
		})
	}()
	return nil
}

func (c *appClient) PipeClose(id string, pipe pipeutil.Pipe) error {
	//todo imp close func
	return nil
}

// 如果f方法操作失败，并且返回的错误为空，那么直接进行重试操作。如果返回的错误不为空，那么设置backoff时长的定时器，计时结束后，再次重试操作。
// 该重试过程以2倍形式逐渐增加重试等待时间
func (c *appClient) doWithBackOff(msg string, f func() (bool, error)) {
	if !c.retainGoroutine() {
		return
	}
	defer c.releaseGoroutine()

	retryCount := 0

	backOff := initialBackOff
	for {
		if retryCount > 5 {
			glog.Errorf("maximum number 10 of retries exceeded")
			return
		}
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
		retryCount++
		backOff *= 2
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

func (c *appClient) pipeConnection(id string, key string, token string, pipe pipeutil.Pipe, ) (bool, error) {
	newURL, err := util_url.ParseURL(c.url, pipe.PipeType())
	if err != nil {
		return false, err
	}
	newURLStr := fmt.Sprintf(BaseUrl, newURL.Scheme, newURL.Host, key, key, c.clusterId, pipe.PipeType(), token, kube.AgentVersion)
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
	switch pipe.PipeType() {
	case pipeutil.Log:
		if err := pipe.CopyToWebsocketForLog(remote, conn); err != nil {
			glog.Errorf("pipe copy to websocket: %v", err)
			if !IsExpectedWSCloseError(err) {
				return false, err
			}
		}
	case pipeutil.Exec:
		if err := pipe.CopyToWebsocketForExec(remote, conn); err != nil {
			glog.Errorf("pipe copy to websocket: %v", err)
			if !IsExpectedWSCloseError(err) {
				return false, err
			}
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
