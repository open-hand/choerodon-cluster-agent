// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package agent

import (
	"fmt"
	"time"

	"github.com/golang/glog"

	"github.com/choerodon/choerodon-cluster-agent/pkg/agent/model"
)

// How did this update get triggered?
type Cause struct {
	Message string
	User    string
}

// A tagged union for all (both) kinds of update. The type is just so
// we know how to decode the rest of the struct.
type Spec struct {
	Type  string      `json:"type"`
	Cause Cause       `json:"cause"`
	Spec  interface{} `json:"spec"`
}

func (w *workerManager) syncStatus() {
	defer w.wg.Done()

	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	syncTimer := time.NewTimer(10 * time.Second)
	for {
		select {
		case <-w.stop:
			glog.Info("sync loop stopping")
			return
		case <-syncTimer.C:
			for _, envPara := range w.agentInitOps.Envs {
				w.chans.ResponseChan <- newSyncRep(envPara.Namespace)
			}
			syncTimer.Reset(w.statusSyncInterval)
		}
	}

}

func newSyncRep(ns string) *model.Packet {
	return &model.Packet{
		Key:  fmt.Sprintf("env:%s", ns),
		Type: model.StatusSyncEvent,
	}
}
