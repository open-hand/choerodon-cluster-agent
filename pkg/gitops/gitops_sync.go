package gitops

import (
	"context"
	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster/kubernetes"
	resource2 "github.com/choerodon/choerodon-cluster-agent/pkg/cluster/kubernetes/resource"
	"github.com/choerodon/choerodon-cluster-agent/pkg/event"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/resource"
	"github.com/golang/glog"
	"github.com/pkg/errors"
	"strings"
	"sync"
	"time"
)

func (g *GitOps) syncLoop(stop <-chan struct{}, namespace string, stopRepo <-chan struct{}, done *sync.WaitGroup) {
	defer done.Done()

	// We want to sync at least every `SyncInterval`. Being told to
	// sync, or completing a job, may intervene (in which case,
	// reschedule the next sync).
	syncTimer := time.NewTimer(g.syncInterval)

	// Keep track of current HEAD, so we can know when to treat a repo
	// mirror notification as a change. Otherwise, we'll just sync
	// every timer tick as well as every mirror refresh.
	syncHead := ""

	// Ask for a sync
	g.AskForSync(namespace)
	for {
		select {
		case <-stopRepo:
			glog.Infof("env %s sync loop stopping", namespace)
			return
		case <-stop:
			glog.Infof("env %s sync loop stopping", namespace)
			return
		case <-syncTimer.C:
			g.AskForSync(namespace)
		case <-g.gitRepos[namespace].C:
			ctx, cancel := context.WithTimeout(context.Background(), g.gitTimeout)
			newSyncHead, err := g.gitRepos[namespace].Revision(ctx, g.gitConfig.DevOpsTag)
			cancel()
			if err != nil {
				glog.Infof("env: %s %s get DevOps sync head error error: %v", namespace, g.gitRepos[namespace].Origin().URL, err)
				continue
			}
			glog.Infof("env: %s get refreshed event for git repository %s, branch %s, HEAD %s, previous HEAD %s", namespace, g.gitRepos[namespace].Origin().URL, g.gitConfig.Branch, newSyncHead, syncHead)
			if newSyncHead != syncHead {
				syncHead = newSyncHead
				g.AskForSync(namespace)
			}
		case <-g.syncSoon[namespace]:
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			if err := g.doSync(namespace); err != nil {
				glog.Errorf("%s do sync: %v", namespace, err)
			}
			syncTimer.Reset(g.syncInterval)
		}
	}
}

func (g *GitOps) doSync(namespace string) error {
	started := time.Now().UTC()

	// We don't care how long this takes overall, only about not
	// getting bogged down in certain operations, so use an
	// undeadlined context in general.
	ctx := context.Background()

	// checkout a working clone so we can mess around with tags later
	var working *git.Checkout
	{
		var err error
		ctx, cancel := context.WithTimeout(ctx, g.gitTimeout)
		defer cancel()
		working, err = g.gitRepos[namespace].Clone(ctx, g.gitConfig)

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
	manifests := &kubernetes.Manifests{}

	allResources, files, err := manifests.LoadManifests(namespace, working.Dir(), working.ManifestDir())
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
		for _, file := range files {
			commit, err := working.FileLastCommit(ctx, file)
			if err != nil {
				glog.Errorf("get file commit error : v%", err)
				continue
			}
			filesCommits = append(filesCommits, event.FileCommit{File: file, Commit: commit})
			fileCommitMap[file] = commit
		}

	} else {

		ctx, cancel := context.WithTimeout(ctx, g.gitTimeout)
		changedFiles, fileList, err := working.ChangedFiles(ctx, oldTagRev)
		if err == nil && len(changedFiles) > 0 {

			for _, file := range fileList {
				commit, err := working.FileLastCommit(ctx, file)
				if err != nil {
					glog.Errorf("get file commit error : v%", err)
					continue
				}
				filesCommits = append(filesCommits, event.FileCommit{File: file, Commit: commit})
				fileCommitMap[file] = commit
			}
			// We had some changed files, we're syncing a diff
			changedResources, _, err = manifests.LoadManifests(namespace, working.Dir(), changedFiles[0], changedFiles[1:]...)
		}
		cancel()
		if err != nil {
			return errors.Wrap(err, "loading resources from repo")
		}
	}

	for key, k8sResource := range changedResources {

		k8sResourceBuff, err := g.kubeClient.LabelRepoObj(namespace, string(k8sResource.Bytes()), kube.AgentVersion, fileCommitMap[k8sResource.Source()])
		if err != nil {
			glog.Errorf("label of object error ", err)
		} else if k8sResourceBuff != nil {
			obj := resource2.BaseObject{
				SourceName: k8sResource.Source(),
				BytesArray: k8sResourceBuff.Bytes(),
				Meta:       k8sResource.Metas(),
				Kind:       k8sResource.SourceKind(),
			}
			changedResources[key] = obj
		}
	}
	err = Sync(namespace, manifests, allResources, changedResources, g.cluster)

	if err != nil {
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

	for i, _ := range syncErrors {
		if fileCommitMap[syncErrors[i].Path] != "" {
			syncErrors[i].Commit = fileCommitMap[syncErrors[i].Path]
		} else {
			ctx, cancel := context.WithTimeout(ctx, g.gitTimeout)
			commit, err := working.FileLastCommit(ctx, syncErrors[i].Path)
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

	for _, resourceId := range resourceIdList {
		resourceCommit := event.ResourceCommit{
			ResourceId: resourceId.String(),
			File:       changedResources[resourceId.String()].Source(),
			Commit:     fileCommitMap[changedResources[resourceId.String()].Source()],
		}
		resourceCommits = append(resourceCommits, resourceCommit)
	}

	if err := g.LogEvent(event.Event{
		ResourceIDs: resourceIDs.ToSlice(),
		Type:        event.EventSync,
		StartedAt:   started,
		EndedAt:     started,
		Metadata: &event.SyncEventMetadata{
			Commit:          newTagRev,
			Errors:          syncErrors,
			FileCommits:     filesCommits,
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
		ctx, cancel := context.WithTimeout(ctx, g.gitTimeout)
		err := working.MoveSyncTagAndPush(ctx, newTagRev, "Sync pointer")
		cancel()
		if err != nil {
			return err
		}
	}
	// repo refresh
	{
		glog.Infof("%s tag: %s, old: %s, new: %s", namespace, g.gitConfig.SyncTag, oldTagRev, newTagRev)
		ctx, cancel := context.WithTimeout(ctx, g.gitTimeout)
		err := g.gitRepos[namespace].Refresh(ctx)
		cancel()
		return err
	}
}

// Sync synchronises the cluster to the files in a directory
func Sync(namespace string, m cluster.Manifests, repoResources map[string]resource.Resource, changedResources map[string]resource.Resource, clus cluster.Cluster) error {
	// Get a map of resources defined in the cluster
	clusterBytes, err := clus.Export(namespace)

	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := m.ParseManifests(namespace, clusterBytes)
	if err != nil {
		return errors.Wrap(err, "parsing exported resources")
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each application if it is a
	// no-op.
	sync := cluster.SyncDef{}

	for id, res := range clusterResources {
		prepareSyncDelete(repoResources, id, res, &sync)
	}

	for _, res := range changedResources {
		prepareSyncApply(res, &sync)
	}
	return clus.Sync(namespace, sync)
}

// todo: remove
func SyncAll(namespace string, m cluster.Manifests, repoResources map[string]resource.Resource, clus cluster.Cluster) error {
	// Get a map of resources defined in the cluster
	clusterBytes, err := clus.Export(namespace)

	if err != nil {
		return errors.Wrap(err, "exporting resource defs from cluster")
	}
	clusterResources, err := m.ParseManifests(namespace, clusterBytes)
	if err != nil {
		return errors.Wrap(err, "parsing exported resources")
	}

	// Everything that's in the cluster but not in the repo, delete;
	// everything that's in the repo, apply. This is an approximation
	// to figuring out what's changed, and applying that. We're
	// relying on Kubernetes to decide for each application if it is a
	// no-op.
	sync := cluster.SyncDef{}

	for id, res := range clusterResources {
		prepareSyncDelete(repoResources, id, res, &sync)
	}

	for _, res := range repoResources {
		prepareSyncApply(res, &sync)
	}
	return clus.Sync(namespace, sync)
}

func prepareSyncApply(res resource.Resource, sync *cluster.SyncDef) {
	sync.Actions = append(sync.Actions, cluster.SyncAction{
		Apply: res,
	})
}

func prepareSyncDelete(repoResources map[string]resource.Resource, id string, res resource.Resource, sync *cluster.SyncDef) {
	//if len(repoResources) == 0 {
	//	return
	//}
	if _, ok := repoResources[id]; !ok {
		sync.Actions = append(sync.Actions, cluster.SyncAction{
			Delete: res,
		})
	}
}

// todo maybe no work
func (g *GitOps) AskForSync(namespace string) {
	select {
	case g.syncSoon[namespace] <- struct{}{}:
	default:
	}
}

func isUnknownRevision(err error) bool {
	return err != nil &&
		(strings.Contains(err.Error(), "unknown revision or path not in the working tree.") ||
			strings.Contains(err.Error(), "bad revision"))
}
