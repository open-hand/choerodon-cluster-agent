package manager

import "sync"

type Namespaces struct {
	m map[string]bool
	sync.RWMutex
}

func NewNamespaces() *Namespaces {
	return &Namespaces{
		m: map[string]bool{},
	}
}

func (nsSet *Namespaces) Add (ns string)  {
	nsSet.Lock()
	defer nsSet.Unlock()
	nsSet.m[ns] = true
}

func (nsSet *Namespaces) Remove (ns string)  {
	nsSet.Lock()
	defer nsSet.Unlock()
	delete(nsSet.m, ns)
}

func (nsSet *Namespaces) Contain (ns string)  bool {
	nsSet.RLock()
	defer nsSet.RUnlock()
	_, ok := nsSet.m[ns]
	return ok
}

func (nsSet *Namespaces) Set(nsList []string)  {
	nsSet.Lock()
	defer nsSet.Unlock()
	nsSet.m = map[string]bool{}
	for _,ns := range nsList {
		nsSet.m[ns] = true
	}
}

func (nsSet *Namespaces) AddAll(nsList []string)  {
	nsSet.Lock()
	defer nsSet.Unlock()
	for _,ns := range nsList {
		nsSet.m[ns] = true
	}
}

func (nsSet *Namespaces) GetAll() []string {
	nsSet.RLock()
	defer nsSet.RUnlock()
	nsList := []string{}
	for key := range nsSet.m {
		nsList = append(nsList, key)
	}
	return nsList
}