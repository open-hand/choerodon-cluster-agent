package namespace

import (
	"github.com/golang/glog"
	"sync"
)

type Namespaces struct {
	m map[string]bool
	sync.RWMutex
}

func NewNamespaces() *Namespaces {
	ns := &Namespaces{
		m: map[string]bool{},
	}
	// for test-manager auto test
	ns.Add("choerodon-test")
	return ns
}

func (nsSet *Namespaces) Add(ns string) {
	nsSet.Lock()
	defer nsSet.Unlock()
	nsSet.m[ns] = true
}

func (nsSet *Namespaces) Remove(ns string) {
	nsSet.Lock()
	defer nsSet.Unlock()
	delete(nsSet.m, ns)
}

func (nsSet *Namespaces) Contain(ns string) bool {
	defer func() {
		err := recover() // recover() 捕获panic异常，获得程序执行权。
		if err != nil {
			glog.Info(err) // runtime error: index out of range
		}
	}()
	nsSet.RLock()
	defer nsSet.RUnlock()
	_, ok := nsSet.m[ns]
	return ok
}

func (nsSet *Namespaces) Set(nsList []string) {
	nsSet.Lock()
	defer nsSet.Unlock()
	nsSet.m = map[string]bool{}
	for _, ns := range nsList {
		nsSet.m[ns] = true
	}
}

func (nsSet *Namespaces) AddAll(nsList []string) {
	nsSet.Lock()
	defer nsSet.Unlock()
	for _, ns := range nsList {
		nsSet.m[ns] = true
	}
}

func (nsSet *Namespaces) GetAll() []string {
	nsSet.RLock()
	defer nsSet.RUnlock()
	var nsList []string
	for key := range nsSet.m {
		nsList = append(nsList, key)
	}
	return nsList
}
