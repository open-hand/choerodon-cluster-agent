// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package worker

import (
	"context"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/golang/glog"
	"github.com/pkg/errors"

	"github.com/choerodon/choerodon-agent/pkg/cluster"
	"github.com/choerodon/choerodon-agent/pkg/event"
	"github.com/choerodon/choerodon-agent/pkg/git"
	"github.com/choerodon/choerodon-agent/pkg/model"
	"github.com/choerodon/choerodon-agent/pkg/resource"
	c7n_sync "github.com/choerodon/choerodon-agent/pkg/sync"
	"github.com/gin-gonic/gin/json"
	"github.com/choerodon/choerodon-agent/pkg/kube"
	resource2 "github.com/choerodon/choerodon-agent/pkg/cluster/kubernetes/resource"
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

func (w *workerManager) syncLoop(stop <-chan struct{}, done *sync.WaitGroup) {
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
	w.AskForSync()
	for {
		select {
		case <-stop:
			glog.Info("sync loop stopping")
			return
		case <-syncTimer.C:
			w.AskForSync()
		case <-w.gitRepo.C:
			ctx, cancel := context.WithTimeout(context.Background(), gitOpTimeout)
			newSyncHead, err := w.gitRepo.Revision(ctx, w.gitConfig.DevOpsTag)
			cancel()
			if err != nil {
				glog.Infof("%s get DevOps sync head error error: %v", w.gitRepo.Origin().URL, err)
				continue
			}
			glog.Infof("get refreshed event for git repository %s, branch %s, HEAD %s, previous HEAD %s", w.gitRepo.Origin().URL, w.gitConfig.Branch, newSyncHead, syncHead)
			if newSyncHead != syncHead {
				syncHead = newSyncHead
				w.AskForSync()
			}
		case <-w.syncSoon:
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			if err := w.doSync(); err != nil {
				glog.Errorf("do sync: %v", err)
			}
			syncTimer.Reset(w.syncInterval)
		}
	}
}

func (w *workerManager) AskForSync() {
	select {
	case w.syncSoon <- struct{}{}:
	default:
	}
}

func (w *workerManager) doSync() error {
	started := time.Now().UTC()

	// We don't care how long this takes overall, only about not
	// getting bogged down in certain operations, so use an
	// undeadlined context in general.
	ctx := context.Background()

	// checkout a working clone so we can mess around with tags later
	var working *git.Checkout
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		defer cancel()
		working, err = w.gitRepo.Clone(ctx, w.gitConfig)
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

	// Get a map of all resources defined in the repo
	allResources, err := w.manifests.LoadManifests(working.Dir(), working.ManifestDir())
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	var syncErrors []event.ResourceError
	for key,k8sResource := range allResources{

		k8sResourceBuff,err := w.kubeClient.LabelRepoObj(w.namespace, string(k8sResource.Bytes()), kube.AgentVersion)
		if err != nil {
			glog.Errorf("label of object error ",err)
		} else if k8sResourceBuff != nil {
			obj := resource2.BaseObject{
				SourceName :k8sResource.Source(),
				BytesArray: k8sResourceBuff.Bytes(),
				Meta: k8sResource.Metas(),
				Kind: k8sResource.SourceKind(),

			}
			allResources[key] = obj
		}
	}


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
	} else {
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
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
			changedResources, err = w.manifests.LoadManifests(working.Dir(), changedFiles[0], changedFiles[1:]...)
		}
		cancel()
		if err != nil {
			return errors.Wrap(err, "loading resources from repo")
		}
	}

	if err := c7n_sync.Sync(w.manifests, allResources, changedResources, w.cluster); err != nil {
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
			ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
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



	if err := w.LogEvent(event.Event{
		ResourceIDs: resourceIDs.ToSlice(),
		Type:        event.EventSync,
		StartedAt:   started,
		EndedAt:     started,
		Metadata: &event.SyncEventMetadata{
			Commit: newTagRev,
			Errors:  syncErrors,
			FileCommits: filesCommits,
		},
	}); err != nil {
		glog.Errorf("sync log event: %v", err)
	}


	if oldTagRev == newTagRev {
		return nil
	}

	// Move the tag and push it so we know how far we've gotten.
	{
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		err := working.MoveSyncTagAndPush(ctx, newTagRev, "Sync pointer")
		cancel()
		if err != nil {
			return err
		}
	}
	// repo refresh
	{
		glog.Infof("tag: %s, old: %s, new: %s", w.gitConfig.SyncTag, oldTagRev, newTagRev)
		ctx, cancel := context.WithTimeout(ctx, gitOpTimeout)
		err := w.gitRepo.Refresh(ctx)
		cancel()
		return err
	}
}

func (w *workerManager) LogEvent(ev event.Event) error {
	evBytes, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	resp := &model.Response{
		Key:     fmt.Sprintf("env:%s", w.namespace),
		Type:    model.GitOpsSyncEvent,
		Payload: string(evBytes),
	}
	w.responseChan <- resp
	return nil
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
