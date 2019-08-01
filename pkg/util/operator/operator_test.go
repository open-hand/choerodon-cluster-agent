package operator

import "testing"

func TestMgrList(t *testing.T) {
	mgrs := MgrList{}
	mgr01 := &Mgr{
		stopCh: make(chan struct{}, 1),
	}
	mgrs.Add("myns01", mgr01)

	if m := mgrs.Get("myns01"); m == nil {
		t.Fatal("get mgr from list failed")
	}
	if m := mgrs.Get("myns02"); m != nil {
		t.Fatal("get mgr from list wrong")
	}
	if ok := mgrs.Remove("myns02"); ok {
		t.Fatal("delete mgr from list wrong")
	}
	if ok := mgrs.Remove("myns01"); !ok {
		t.Fatal("delete mgr from list wrong")
	}
	if m := mgrs.Get("myns01"); m != nil {
		t.Fatal("get mgr from list wrong")
	}
}
