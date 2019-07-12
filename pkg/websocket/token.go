package websocket

import (
	"fmt"
	"net/http"
)

type Token string

func (t Token) Set(req *http.Request) {
	if string(t) != "" {
		req.Header.Set("Authorization", fmt.Sprintf("bearer %s", t))
	}
}
