package ws

import (
	"crypto/rand"
	"flag"
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/stretchr/testify/assert"
	"math/big"
	"net/http"
	"strconv"
	"sync/atomic"
	"testing"
	"time"
)

type mockServer struct {
	port       int
	conn       *websocket.Conn
	pingNumber int32
	srv        *http.Server
	reply      bool
	stopCh     chan struct{}
}

func (this *mockServer) getPort() int {
	return this.port
}

func (this *mockServer) listener() {

	mux := http.NewServeMux()

	this.srv = &http.Server{
		Addr:    ":" + strconv.Itoa(this.port),
		Handler: mux,
	}

	mux.HandleFunc("/ws", this.upgrader)

	go func() {

		glog.Infof("Start listener port:%d.", this.port)
		glog.Error(this.srv.ListenAndServe())

	}()
}

func (this *mockServer) upgrader(w http.ResponseWriter, r *http.Request) {
	u := websocket.Upgrader{
		CheckOrigin: func(r *http.Request) bool {
			return true
		},
	}
	conn, err := u.Upgrade(w, r, nil)
	if err != nil {
		glog.Errorln(err)
		return
	}

	this.conn = conn

	this.conn.SetPingHandler(func(msg string) error {

		atomic.AddInt32(&this.pingNumber, 1)

		if this.reply {

			glog.Info("Respond to Pong message.")

			return this.conn.WriteControl(websocket.PongMessage, []byte{}, time.Now().Add(3*time.Second))

		} else {

			glog.Info("Received Ping message, but is now set not to respond to Pong message.")

			return nil

		}

	})

	go func() {
		for {
			select {
			case <-this.stopCh:
				break
			default:
				{
					_, _, err := this.conn.ReadMessage()
					if err != nil {
						goto Exit
					}
				}
			}
		}
	Exit:
	}()
}

func (this *mockServer) close() {

	close(this.stopCh)

	glog.Error(this.srv.Shutdown(nil))

}

func randInt64(min, max int64) int64 {
	maxBigInt := big.NewInt(max)
	i, _ := rand.Int(rand.Reader, maxBigInt)
	if i.Int64() < min {
		randInt64(min, max)
	}
	return i.Int64()
}

func newServer(reply bool) *mockServer {
	s := &mockServer{
		port:   int(randInt64(3000, 30000)),
		reply:  reply,
		stopCh: make(chan struct{}),
	}

	return s
}

func TestMain(m *testing.M) {
	flag.Set("alsologtostderr", "true")
	flag.Set("log_dir", "/tmp")
	flag.Set("v", "3")
	flag.Parse()

	m.Run()

}

/**
no poing response, try 3 times.
*/
func TestNotPong(t *testing.T) {

	server := newServer(false)
	server.listener()
	defer func() {
		server.close()
	}()

	time.Sleep(2 * time.Second)

	url := "ws://127.0.0.1:" + strconv.Itoa(server.getPort()) + "/ws"
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Second,
	}
	client, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}

	var closed bool
	hc, hcErr := NewHealthCheck(&Conf{
		ReadLimit:            1024 * 1024,
		ConnectionTimeout:    time.Second,
		WriteTimeout:         time.Hour,
		HealthCheckDuration:  time.Second,
		HealthCheckTimeout:   time.Second,
		HealthCheckTryNumber: 3,
	}, client, func() {
		client.Close()
		closed = true
	})

	if hcErr != nil {
		t.Fatal(hcErr)
	}

	time.Sleep(6 * time.Second)

	hc.Stop()

	assert.True(t, closed)
	assert.Equal(t, int32(3), server.pingNumber)
	assert.Equal(t, int32(3), hc.hasTryNumber)
	assert.False(t, hc.IsChecking())

}

// server response poing, expect check 4 times.
func TestServerPingCorrectly(t *testing.T) {

	server := newServer(true)

	server.listener()

	defer func() {
		server.close()
	}()

	time.Sleep(2 * time.Second)

	url := "ws://127.0.0.1:" + strconv.Itoa(server.getPort()) + "/ws"
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Second,
	}
	client, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}

	var hc *HealthCheck
	var hcErr error
	client.SetPongHandler(func(appData string) error {
		return hc.OnPone([]byte(appData))
	})

	go func() {
		for {
			_, _, e := client.ReadMessage()
			if e != nil {
				break
			}

		}
	}()

	var closed bool
	hc, hcErr = NewHealthCheck(&Conf{
		ReadLimit:            1024 * 1024,
		ConnectionTimeout:    time.Second,
		WriteTimeout:         time.Hour,
		HealthCheckDuration:  time.Second,
		HealthCheckTimeout:   3 * time.Second,
		HealthCheckTryNumber: 3,
	}, client, func() {
		client.Close()
		closed = true
	})

	if hcErr != nil {
		t.Fatal(hcErr)
	}

	var waitNumber int32
	for {

		if waitNumber >= 2000 {
			t.Fatal("The wait timeout did not meet the expected condition.")
		}

		if server.pingNumber > 3 {
			break
		} else {
			waitNumber++
			time.Sleep(time.Second)
		}
	}

	hc.Stop()

	assert.False(t, closed)
	assert.Equal(t, int32(0), hc.hasTryNumber)
	assert.Equal(t, int32(4), server.pingNumber)
	assert.False(t, hc.IsChecking())

}

// receive pong at err time. This is not a check status.
func TestRecvicePongAtNoErrTime(t *testing.T) {
	server := newServer(true)

	server.listener()

	defer func() {
		server.close()
	}()

	time.Sleep(2 * time.Second)

	url := "ws://127.0.0.1:" + strconv.Itoa(server.getPort()) + "/ws"
	dialer := websocket.Dialer{
		HandshakeTimeout: time.Second,
	}
	client, _, err := dialer.Dial(url, nil)
	if err != nil {
		t.Fatal(err)
	}

	go func() {
		for {
			_, _, e := client.ReadMessage()
			if e != nil {
				break
			}

		}
	}()

	hc, hcErr := NewHealthCheck(&Conf{
		ReadLimit:            1024 * 1024,
		ConnectionTimeout:    time.Second,
		WriteTimeout:         time.Hour,
		HealthCheckDuration:  time.Hour, // Too long, this test will not be detected.
		HealthCheckTimeout:   3 * time.Second,
		HealthCheckTryNumber: 3,
	}, client, nil)

	if hcErr != nil {
		t.Fatal(hcErr)
	}

	hc.OnPone(nil)

	time.Sleep(2 * time.Millisecond)

	assert.Equal(t, 0, len(hc.pongCh))

	hc.startCheck()

	hc.OnPone(nil)

	time.Sleep(2 * time.Millisecond)

	assert.Equal(t, 1, len(hc.pongCh))

	hc.Stop()
}
