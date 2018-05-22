package http

import (
	"net/http"
	_ "net/http/pprof"
)

type server struct {
	addr string
}

func NewServer(addr string) *server {
	s := &server{
		addr: addr,
	}
	return s
}

func (s *server) Run() {
	http.ListenAndServe(s.addr, nil)
}
