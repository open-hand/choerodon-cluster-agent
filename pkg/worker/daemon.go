// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package worker

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	resource2 "github.com/choerodon/choerodon-cluster-agent/pkg/cluster/kubernetes/resource"
	"github.com/choerodon/choerodon-cluster-agent/pkg/event"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/model"
	"github.com/choerodon/choerodon-cluster-agent/pkg/resource"
	c7n_sync "github.com/choerodon/choerodon-cluster-agent/pkg/sync"
	"github.com/gin-gonic/gin/json"
	"sync"
)

type note struct {
	Spec Spec `json:"spec"`
}

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



func (w *workerManager) syncLoop(stop <-chan struct{}, namespace string, stopRepo  chan<- struct{}, done *sync.WaitGroup) {
	defer done.Done()

	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	syncTimer := time.NewTimer(w.syncInterval)

	// Keep track of current HEAD, so we can know when to treat a repo
	// mirror notification as a change. Otherwise, we'll just sync
	// every timer tick as well as every mirror refresh.
	syncHead := ""

	// Ask for a sync
	w.AskForSync(namespace)
	for {
		select {
		case <-stop:
			glog.Info("sync loop stopping")
			return
		case <-syncTimer.C:
			w.AskForSync(namespace)
		case <-w.gitRepos[namespace].C:
			ctx, cancel := context.WithTimeout(context.Background(), w.gitTimeout)
			newSyncHead, err := w.gitRepos[namespace].Revision(ctx, w.gitConfig.DevOpsTag)
			cancel()
			if err != nil {
				glog.Infof("env: %s %s get DevOps sync head error error: %v", namespace, w.gitRepos[namespace].Origin().URL, err)
				continue
			}
			glog.Infof("env: %s get refreshed event for git repository %s, branch %s, HEAD %s, previous HEAD %s", namespace, w.gitRepos[namespace].Origin().URL, w.gitConfig.Branch, newSyncHead, syncHead)
			if newSyncHead != syncHead {
				syncHead = newSyncHead
				w.AskForSync(namespace)
			}
		case <-w.syncSoon[namespace]:
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			if err := w.doSync(namespace); err != nil {
				glog.Errorf("%s do sync: %v", namespace, err)
			}
			syncTimer.Reset(w.syncInterval)
		}
	}
}

func (w *workerManager) AskForSync(namespace string) {
	select {
	case w.syncSoon[namespace] <- struct{}{}:
	default:
	}
}

func (w *workerManager) doSync(namespace string) error {started := time.Now().UTC()

	// We don't care how long this takes overall, only about not
	// getting bogged down in certain operations, so use an
	// undeadlined context in general.
	ctx := context.Background()

	// checkout a working clone so we can mess around with tags later
	var working *git.Checkout
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, w.gitTimeout)
		defer cancel()
		working, err = w.gitRepos[namespace].Clone(ctx, w.gitConfig)

		if err != nil {
			return err
		}
		defer working.Clean()
	}

	// For comparison later.
	oldTagRev, err := working.SyncRevision(ctx)
	if err != nil && !isUnknownRevision(err) {
		return err
	}

	newTagRev, err := working.DevOpsSyncRevision(
		ctx)
	if err != nil {
		return err
	}

	// Get a map of all resources defined in  therepo
	allResources,files, err := w.manifests.LoadManifests(namespace, working.Dir(), working.ManifestDir())
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	var syncErrors []event.ResourceError



	var initialSync bool

	if oldTagRev == "" {
		initialSync = true
	}

	// Figure out which service IDs changed in this release
	changedResources := map[string]resource.Resource{}
	filesCommits := make([]event.FileCommit, 0)
	fileCommitMap := map[string]string{}



	if initialSync {
		// no synctag, We are syncing everything from scratch
		changedResources = allResources
		for _,file := range files {
			commit, err := working.FileLastCommit(ctx, file)
			if err != nil {
				glog.Errorf("get file commit error : v%", err)
				continue
			}
			filesCommits = append(filesCommits, event.FileCommit{File:file, Commit: commit})
			fileCommitMap[file] = commit
		}

	} else {

		ctx, cancel := context.WithTimeout(ctx, w.gitTimeout)
		changedFiles,fileList, err := working.ChangedFiles(ctx, oldTagRev)
		if err == nil && len(changedFiles) > 0 {

			for _,file := range fileList {
				commit, err := working.FileLastCommit(ctx, file)
				if err != nil {
					glog.Errorf("get file commit error : v%", err)
					continue
				}
				filesCommits = append(filesCommits, event.FileCommit{File:file, Commit: commit})
				fileCommitMap[file] = commit
			}
			// We had some changed files, we're syncing a diff
			changedResources,_, err = w.manifests.LoadManifests(namespace, working.Dir(), changedFiles[0], changedFiles[1:]...)
		}
		cancel()
		if err != nil {
			return errors.Wrap(err, "loading resources from repo")
		}
	}

	for key,k8sResource := range changedResources{

		k8sResourceBuff,err := w.kubeClient.LabelRepoObj(namespace, string(k8sResource.Bytes()), kube.AgentVersion, fileCommitMap[k8sResource.Source()])
		if err != nil {
			glog.Errorf("label of object error ",err)
		} else if k8sResourceBuff != nil {
			obj := resource2.BaseObject{
				SourceName :k8sResource.Source(),
				BytesArray: k8sResourceBuff.Bytes(),
				Meta: k8sResource.Metas(),
				Kind: k8sResource.SourceKind(),
			}
			changedResources[key] = obj
		}
	}

	if err := c7n_sync.Sync(namespace, w.manifests, allResources, changedResources, w.cluster); err != nil {
		glog.Errorf("sync: %v", err)
		switch syncerr := err.(type) {
		case cluster.SyncError:
			for _, e := range syncerr {
				syncErrors = append(syncErrors, event.ResourceError{
					ID:    e.ResourceID(),
					Path:  e.Source(),
					Error: e.Error.Error(),
				})
			}
		default:
			return err
		}
	}

	// update notes and emit events for applied commits


	for i,_ := range syncErrors {
		if fileCommitMap[syncErrors[i].Path] != "" {
			syncErrors[i].Commit = fileCommitMap[syncErrors[i].Path]
		}else {
			ctx, cancel := context.WithTimeout(ctx, w.gitTimeout)
			commit,err := working.FileLastCommit(ctx, syncErrors[i].Path)
			if err != nil {
				glog.Errorf("get file commit error : v%", err)
			} else {
				syncErrors[i].Commit = commit
			}
			cancel()
		}
	}

	resourceIDs := resource.ResourceIDSet{}
	for _, r := range changedResources {
		resourceIDs.Add([]resource.ResourceID{r.ResourceID()})
	}
	resourceCommits := make([]event.ResourceCommit, 0)

	resourceIdList := resourceIDs.ToSlice()

	for _,resourceId := range resourceIdList{
		resourceCommit := event.ResourceCommit{
			ResourceId: resourceId.String(),
			File: changedResources[resourceId.String()].Source(),
			Commit: fileCommitMap[changedResources[resourceId.String()].Source()],

		}
		resourceCommits = append(resourceCommits, resourceCommit)
	}


	if err := w.LogEvent(event.Event{
		ResourceIDs: resourceIDs.ToSlice(),
		Type:        event.EventSync,
		StartedAt:   started,
		EndedAt:     started,
		Metadata: &event.SyncEventMetadata{
			Commit: newTagRev,
			Errors:  syncErrors,
			FileCommits: filesCommits,
			ResourceCommits: resourceCommits,
		},
	}, namespace); err != nil {
		glog.Errorf("sync log event: %v", err)
	}


	if oldTagRev == newTagRev {
		return nil
	}

	// Move the tag and push it so we know how far we've gotten.
	{
		ctx, cancel := context.WithTimeout(ctx, w.gitTimeout)
		err := working.MoveSyncTagAndPush(ctx, newTagRev, "Sync pointer")
		cancel()
		if err != nil {
			return err
		}
	}
	// repo refresh
	{
		glog.Infof("%s tag: %s, old: %s, new: %s",namespace, w.gitConfig.SyncTag, oldTagRev, newTagRev)
		ctx, cancel := context.WithTimeout(ctx, w.gitTimeout)
		err := w.gitRepos[namespace].Refresh(ctx)
		cancel()
		return err
	}
}

func (w *workerManager) LogEvent(ev event.Event, namespace string) error {
	evBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp := &model.Packet{
		Key:     fmt.Sprintf("env:%s", namespace),
		Type:    model.GitOpsSyncEvent,
		Payload: string(evBytes),
	}
	glog.Infof("%s git_ops_sync_event:\n%s", resp.Key, resp.Payload)
	w.chans.ResponseChan <- resp
	return nil
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}

func (w *workerManager)syncStatus(stop <-chan struct{}, done *sync.WaitGroup)  {
	defer done.Done()

	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	syncTimer := time.NewTimer(10 * time.Second)
	for{
		select {
		case <- stop:
			glog.Info("sync loop stopping")
			return
		case <- syncTimer.C:
			for _, envPara := range w.agentInitOps.Envs {
				w.chans.ResponseChan <- newSyncRep(envPara.Namespace)
			}
			syncTimer.Reset(w.statusSyncInterval)
		}
	}

}

func newSyncRep(ns string) *model.Packet {
	return &model.Packet{
		Key:     fmt.Sprintf("env:%s", ns),
		Type:    model.StatusSyncEvent,
	}
}
