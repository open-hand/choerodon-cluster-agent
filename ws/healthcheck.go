package ws

import (
	"github.com/golang/glog"
	"github.com/gorilla/websocket"
	"github.com/pkg/errors"
	"sync"
	"sync/atomic"
	"time"
)

var empty = []byte{}

/*
Health detection is achieved.
Use websocket's pong message to check the availability of the current link.
In order to deal with network latency, the pong message is merged as follows.

1. Send the ping.
2. Wait for poing timeout (in fact, the server has responded, but only because of network delay)
3. Launch ping again. (pong that arrived before this ping will be ignored.)
4. It is possible to receive two pong at this time, and this is considered to be the same response for these two pong.
*/
type HealthCheck struct {
	conn            *websocket.Conn
	conf            *Conf
	lastRecvice     time.Time
	lastRecviceLock *sync.RWMutex
	tick            *time.Ticker
	stopCh          chan struct{}
	pongCh          chan struct{}
	checking        int32
	hasTryNumber    int32
	terminator      func()
	over            bool
}

func NewHealthCheck(conf *Conf, conn *websocket.Conn, t func()) (*HealthCheck, error) {
	hc := &HealthCheck{
		conn:            conn,
		conf:            conf,
		stopCh:          make(chan struct{}),
		pongCh:          make(chan struct{}, 10),
		hasTryNumber:    0,
		tick:            time.NewTicker(conf.HealthCheckDuration),
		checking:        0,
		lastRecviceLock: new(sync.RWMutex),
		terminator:      t,
		lastRecvice:     time.Now(),
	}

	if t == nil {
		hc.terminator = func() {
			// do nothing.
		}
	}

	/**
	如果当前正在检查中那么不再进行忽略此次检查.因为 tick 计时器一直在运行的.有可能上次检查还未结束.
	如果最后接受消息时间没有超过指定时间,那么忽略此次.
	如果检查超出次数上限了那么直接判定生命检查失败.
	*/
	go func(hc *HealthCheck) {

		for {
			if hc.over {
				break
			}

			select {
			case <-hc.tick.C:
				{
					if !hc.IsChecking() {
						if time.Now().Sub(hc.readUpdateTime()) >= conf.HealthCheckDuration {

							hc.startCheck()

							glog.V(3).Infof("Send a Ping message to %s.", hc.conn.RemoteAddr().String())

							err := hc.conn.WriteControl(websocket.PingMessage, empty, time.Now().Add(hc.conf.WriteTimeout))
							if err != nil {

								glog.Error(err)

								hc.kill()

							} else {

								if !hc.waitPong(hc.conf.HealthCheckTimeout) {
									hc.hasTryNumber++

									glog.V(1).Infof("The health test failed at the %d.", hc.hasTryNumber)

									if hc.isOverrun() {
										hc.kill()
									}

								} else {
									hc.hasTryNumber = 0
									glog.V(3).Info("Health check pass.")
								}

							}

							hc.stopCheck()
						}
					} else {

						glog.V(3).Info("Health examination in progress...")

					}

				}
			case <-hc.stopCh:
				{
					hc.tick.Stop()

					glog.Info("Health check closed!")

					break
				}
			}
		}

	}(hc)

	return hc, nil
}

func (this *HealthCheck) OnRecvice(data []byte) error {

	if err := this.checkOver(); err != nil {
		return err
	}

	this.doUpdateTime()

	return nil
}

// Reply to ping.
func (this *HealthCheck) OnPing(data []byte) error {
	glog.V(3).Info("Ping message received.")

	if err := this.checkOver(); err != nil {
		return err
	}

	this.doUpdateTime()

	return this.conn.WriteControl(websocket.PongMessage, data, time.Now().Add(this.conf.WriteTimeout))
}

// recvice pong.
func (this *HealthCheck) OnPone(data []byte) error {
	glog.V(3).Info("Pong message received.")

	if err := this.checkOver(); err != nil {
		return err
	}

	this.doUpdateTime()

	// Prevent blocking.so use go.
	go func() {

		if this.IsChecking() {

			this.pongCh <- struct{}{}

		} else {

			glog.V(2).Info("Pong message is received to check timeout. Ignore it.")

		}
	}()

	return nil
}

func (this *HealthCheck) Stop() {
	if !this.over {
		close(this.stopCh)

		this.tick.Stop()

		this.over = true

		glog.Info("Health check stop!")
	}
}

func (this *HealthCheck) checkOver() error {
	if this.over {
		return errors.New("It's closed.")
	}

	return nil
}

func (this *HealthCheck) doUpdateTime() {
	this.lastRecviceLock.Lock()
	defer this.lastRecviceLock.Unlock()
	this.lastRecvice = time.Now()
}

func (this *HealthCheck) readUpdateTime() time.Time {
	this.lastRecviceLock.RLock()
	defer this.lastRecviceLock.RUnlock()
	return this.lastRecvice
}

// wait pong or timeout.
// true ok, false timeout.
func (this *HealthCheck) waitPong(timeout time.Duration) bool {
	select {
	case <-this.pongCh:
		{
			// clear pongCh,
			clearChann(this.pongCh)

			return true
		}
	case <-time.After(timeout):
		return false
	}
}

func (this *HealthCheck) isOverrun() bool {
	if this.hasTryNumber >= this.conf.HealthCheckTryNumber {
		return true
	} else {
		return false
	}
}

func (this *HealthCheck) kill() {
	this.Stop()
	glog.Infof("Health check failed %d times, so destroy the connection.", this.conf.HealthCheckTryNumber)
	this.terminator()
}

func (this *HealthCheck) IsChecking() bool {
	return atomic.LoadInt32(&this.checking) > 0
}

func (this *HealthCheck) startCheck() {
	atomic.StoreInt32(&this.checking, 1)
}

func (this *HealthCheck) stopCheck() {
	atomic.StoreInt32(&this.checking, 0)
}

func clearChann(pongCh chan struct{}) {
	for {
		if len(pongCh) > 0 {
			<-pongCh
		} else {
			break
		}
	}
}
