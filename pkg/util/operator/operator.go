package operator

import (
	crmanager "sigs.k8s.io/controller-runtime/pkg/manager"
)

type Mgr struct {
	crmanager.Manager
	stopCh chan struct{}
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

func (ms *MgrList) AddStop(namespace string, manager crmanager.Manager, stopCh chan struct{}) bool {
	if _, ok := (*ms)[namespace]; ok && (*ms)[namespace] != nil {
		return false
	}
	mgr := &Mgr{
		stopCh:  stopCh,
		Manager: manager,
	}
	(*ms)[namespace] = mgr
	return true
}

func (ms *MgrList) Remove(namespace string) bool {
	var mgr *Mgr
	var ok bool
	if mgr, ok = (*ms)[namespace]; !ok {
		return false
	}
	close(mgr.stopCh)
	delete(*ms, namespace)
	return true
}

func (ms *MgrList) StopAll() bool {
	for _, m := range *ms {
		close(m.stopCh)
	}
	return true
}
