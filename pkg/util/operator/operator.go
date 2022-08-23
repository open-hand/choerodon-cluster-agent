package operator

import (
	"context"
	crmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

type Mgr struct {
	crmanager.Manager
	ctx        context.Context
	cancelFunc context.CancelFunc
}

type MgrList map[string]*Mgr

func (ms *MgrList) Get(namespace string) *Mgr {
	return (*ms)[namespace]
}

func (ms *MgrList) Add(namespace string, mgr *Mgr) bool {
	if _, ok := (*ms)[namespace]; ok && (*ms)[namespace] != nil {
		return false
	}
	(*ms)[namespace] = mgr
	return true
}

func (ms *MgrList) IsExist(namespace string) bool {
	if _, ok := (*ms)[namespace]; ok && (*ms)[namespace] != nil {
		return true
	}
	return false
}

func (ms *MgrList) AddStop(namespace string, manager crmanager.Manager) bool {
	if _, ok := (*ms)[namespace]; ok && (*ms)[namespace] != nil {
		return false
	}
	ctx, cancelFunc := context.WithCancel(context.TODO())
	mgr := &Mgr{
		ctx:        ctx,
		Manager:    manager,
		cancelFunc: cancelFunc,
	}
	(*ms)[namespace] = mgr
	return true
}

func (ms *MgrList) GetCtx(namespace string) context.Context {
	return (*ms)[namespace].ctx
}

func (ms *MgrList) Remove(namespace string) bool {
	var mgr *Mgr
	var ok bool
	if mgr, ok = (*ms)[namespace]; !ok {
		return false
	}
	mgr.cancelFunc()
	delete(*ms, namespace)
	return true
}

func (ms *MgrList) StopAll() bool {
	for _, m := range *ms {
		m.cancelFunc()
	}
	return true
}
