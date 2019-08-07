package ws

import (
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/gorilla/websocket"
)

func dial(urlStr string, token Token, timeout time.Duration) (*websocket.Conn, error) {
	req, err := http.NewRequest("GET", urlStr, nil)
	if err != nil {
		return nil, fmt.Errorf("constructing request %s: %v", urlStr, err)
	}

	token.Set(req)

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: timeout,
	}

	conn, _, err := dialer.Dial(urlStr, req.Header)

	if err != nil {
		return nil, fmt.Errorf("dial error %s: %v", urlStr, err)
	}

	return conn, nil
}

func dialWS(urlStr string, requestHeader http.Header, timeout time.Duration) (*websocket.Conn, *http.Response, error) {

	dialer := websocket.Dialer{
		Proxy:            http.ProxyFromEnvironment,
		HandshakeTimeout: timeout,
	}
	conn, resp, err := dialer.Dial(urlStr, requestHeader)

	if err != nil {
		return nil, resp, fmt.Errorf("dial error %s: %v", urlStr, err)
	}
	return conn, resp, nil
}

func IsExpectedWSCloseError(err error) bool {
	return err == io.EOF || err == io.ErrClosedPipe || websocket.IsCloseError(err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
		websocket.CloseAbnormalClosure,
	)
}
