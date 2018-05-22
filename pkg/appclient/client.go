package appclient

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/hashicorp/go-cleanhttp"

	"github.com/choerodon/choerodon-agent/pkg/common"
	"github.com/choerodon/choerodon-agent/pkg/model"
	util_url "github.com/choerodon/choerodon-agent/pkg/util/url"
)

const (
	// Time allowed to write a message to the peer.
	WriteWait      = 10 * time.Second
	initialBackOff = 1 * time.Second
	maxBackOff     = 60 * time.Second
)

type AppClient interface {
	Start(stopCh <-chan struct{}) error
	PipeConnection(pipeID string, pipe common.Pipe) error
	PipeClose(pipeID string, pipe common.Pipe) error
	Stop()
}

type appClient struct {
	url        *url.URL
	token      Token
	commandCh  chan<- *model.Command
	responseCh <-chan *model.Response
	conn       *websocket.Conn

	quit           chan struct{}
	mtx            sync.Mutex
	client         *http.Client
	backgroundWait sync.WaitGroup
	pipeConns      map[string]*websocket.Conn
}

func (c *appClient) Start(stopCh <-chan struct{}) error {
	go c.loop(stopCh)

	glog.Info("Started agent")
	<-stopCh
	glog.Info("Shutting down agent")

	return nil
}

func (c *appClient) loop(stopCh <-chan struct{}) {
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
			}
			time.Sleep(backOff)
		case <-stopCh:
			return
		}
	}
}

func (c *appClient) connect() error {
	glog.V(1).Info("start connect to devops service")
	var err error
	c.conn, err = dial(c.url.String(), c.token)
	if err != nil {
		return err
	}
	defer func() {
		glog.V(1).Info("exit connect")
		c.conn.Close()
	}()

	done := make(chan struct{})
	go func() {
		defer close(done)

		c.conn.SetPingHandler(nil)
		for {
			var command model.Command
			err := c.conn.ReadJSON(&command)
			if err != nil {
				if !websocket.IsCloseError(err, websocket.CloseNoStatusReceived) {
					glog.Error(err)
				}
				break
			}
			glog.V(1).Info("receive command: ", command)
			c.commandCh <- &command
		}
	}()

	for {
		select {
		case <-done:
			return nil
		case resp, ok := <-c.responseCh:
			c.conn.SetWriteDeadline(time.Now().Add(WriteWait))
			if !ok {
				c.conn.WriteMessage(websocket.CloseMessage, []byte{})
				return nil
			}
			content, _ := json.Marshal(resp)
			glog.V(1).Info("send response: ", string(content))
			err := c.conn.WriteJSON(resp)
			if err != nil {
				glog.Error(err)
			}
		}
	}
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

func (c *appClient) Stop() {
	c.mtx.Lock()
	close(c.quit)
	c.mtx.Unlock()

	c.backgroundWait.Wait()
}

func (c *appClient) PipeConnection(id string, pipe common.Pipe) error {
	go func() {
		glog.Infof("Pipe %s connection to %s starting", id, c.url)
		defer glog.Infof("Pipe %s connection to %s exiting", id, c.url)
		c.doWithBackOff(id, func() (bool, error) {
			return c.pipeConnection(id, pipe)
		})
	}()
	return nil
}

func (c *appClient) PipeClose(id string, pipe common.Pipe) error {
	//newURL, err := util_url.ParseURL(c.url, pipe.PipeType())
	//if err != nil {
	//	return err
	//}
	//newURL.Scheme = "http"
	//newURLStr := fmt.Sprintf("%s.%s:%s", newURL.String(), pipe.PipeType(), id)
	//req, err := http.NewRequest(http.MethodDelete, newURLStr, nil)
	//if err != nil {
	//	return err
	//}
	//resp, err := c.client.Do(req)
	//if err != nil {
	//	return err
	//}
	//resp.Body.Close()
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

func (c *appClient) pipeConnection(id string, pipe common.Pipe) (bool, error) {
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

func NewClient(
	t Token,
	endpoint string,
	commandChan chan<- *model.Command,
	responseChan <-chan *model.Response) (AppClient, error) {
	if endpoint == "" {
		return nil, fmt.Errorf("no upstream URL given")
	}

	endpointURL, err := url.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("parsing endpoint %s: %v", endpoint, err)
	}

	httpClient := cleanhttp.DefaultClient()

	c := &appClient{
		url:        endpointURL,
		token:      t,
		commandCh:  commandChan,
		responseCh: responseChan,
		quit:       make(chan struct{}),
		client:     httpClient,
		pipeConns:  make(map[string]*websocket.Conn),
	}

	return c, nil
}
