package model

import (
	"context"
	"github.com/golang/glog"
	"net/http"
)

var HealthyProbServer *http.Server

func StartHealthyProbServer() {
	mux := http.ServeMux{}
	mux.HandleFunc("/healthy", OK)
	HealthyProbServer = &http.Server{Addr: HealthyListen, Handler: &mux}
	err := HealthyProbServer.ListenAndServe()
	if err != nil {
		glog.Error(err)
	}
}

func StopHealthyProbServer() {
	if HealthyProbServer != nil {
		err := HealthyProbServer.Shutdown(context.Background())
		if err != nil {
			glog.Error(err)
		}
		HealthyProbServer = nil
	}

}

func OK(resp http.ResponseWriter, req *http.Request) {
	resp.WriteHeader(http.StatusOK)
}
