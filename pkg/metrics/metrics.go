package metrics

import (
	"github.com/golang/glog"
	"sync"
)

type Metrics interface {
	Run(stopCh <-chan struct{}) error
}

var Funcs []Metrics

func Register(m Metrics) {
	Funcs = append(Funcs, m)
}

func Run(stopCh <-chan struct{}) {
	glog.V(1).Info("start catch metrics")
	wg := sync.WaitGroup{}
	for _, m := range Funcs {
		wg.Add(1)
		go func() {
			if err := m.Run(stopCh); err != nil {
				glog.V(1).Info(err)
			}
			wg.Done()
		}()
	}
	wg.Wait()
}
