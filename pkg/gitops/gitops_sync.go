package gitops

import (
	"context"
	"github.com/choerodon/choerodon-cluster-agent/pkg/git"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kube"
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes"
	resource2 "github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes/resource"
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

	//NewTimer 创建一个 Timer，它会在最少过去时间段 d 后到期，向其自身的 C 字段发送当时的时间
	// 周期定时器，倒计时结束后会发出信号，让agent自动同步远程代码
	syncTimer := time.NewTimer(g.syncInterval)

	// Keep track of current HEAD, so we can know when to treat a repo
	// mirror notification as a change. Otherwise, we'll just sync
	// every timer tick as well as every mirror refresh.
	// 这个表示agent已经同步过最新的commit
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
			ctx, cancel := context.WithTimeout(context.Background(), git.Timeout)
			// 获得devops-sync这个tag最新的提交commit,作为最新的提交记录
			newSyncHead, err := g.gitRepos[namespace].Revision(ctx, g.gitConfig.DevOpsTag)
			cancel()
			if err != nil {
				glog.Infof("env: %s %s get DevOps sync head error error: %v", namespace, g.gitRepos[namespace].Origin().URL, err)
				continue
			}
			glog.Infof("env: %s get refreshed event for git repository %s, branch %s, HEAD %s, previous HEAD %s", namespace, g.gitRepos[namespace].Origin().URL, g.gitConfig.Branch, newSyncHead, syncHead)
			//如果agent同步到commit和远程仓库的devops-sync这个tag的最新提交不一致，表明配置库有变化，需要进行同步操作
			if newSyncHead != syncHead {
				// 将agent的同步commit更新为最新的，即与devops-sync这个tag的最新提交
				syncHead = newSyncHead
				// 让agent开始同步
				g.AskForSync(namespace)
			}
		case <-g.syncSoon[namespace]:
			// 猜测 如果syncTimer停止失败，那么timer会继续倒计时，等待timer计时结束
			if !syncTimer.Stop() {
				select {
				case <-syncTimer.C:
				default:
				}
			}
			if err := g.doSync(namespace); err != nil {
				glog.Errorf("%s do sync: %v", namespace, err)
			}
			// 再次开启倒计时
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
		ctx, cancel := context.WithTimeout(ctx, git.Timeout)
		defer cancel()
		// 把devops-sync这个tag下的配置库拉下来
		working, err = g.gitRepos[namespace].Clone(ctx, g.gitConfig)
		if err != nil {
			return err
		}
		defer working.Clean()
	}

	// For comparison later.
	// 就是agent同步过的最新的commit
	oldTagRev, err := working.SyncRevision(ctx)
	if err != nil && !isUnknownRevision(err) {
		return err
	}

	// 配置库devops-sync最新的commit,即是配置库的最新commit
	newTagRev, err := working.DevOpsSyncRevision(
		ctx)
	if err != nil {
		return err
	}

	// Get a map of all resources defined in  therepo
	manifests := &kubernetes.Manifests{}

	// 载入配置库中的所有资源
	allResources, files, err := manifests.LoadManifests(namespace, working.Dir(), working.ManifestDir())
	if err != nil {
		return errors.Wrap(err, "loading resources from repo")
	}

	var syncErrors []ResourceError

	var initialSync bool

	// oldTagRev为空表示这是第一次同步配置库
	if oldTagRev == "" {
		initialSync = true
	}

	// Figure out which service IDs changed in this release

	// 与上一次同步的结果进行比较，得出还未同步的文件
	changedResources := map[string]resource.Resource{}

	filesCommits := make([]FileCommit, 0)
	fileCommitMap := map[string]string{}

	if initialSync {
		// 配置库是第一次进行同步，所有的资源都纳入需要同步的范畴
		changedResources = allResources
		for _, file := range files {
			// 获得指定文件的最后一次提交记录
			commit, err := working.FileLastCommit(ctx, file)
			if err != nil {
				glog.Errorf("get file commit error : v%", err)
				continue
			}
			filesCommits = append(filesCommits, FileCommit{File: file, Commit: commit})
			fileCommitMap[file] = commit
		}
	} else {
		// 不是第一次同步，则需要与上一次同步的结果进行比较，得出还未同步的文件
		ctx, cancel := context.WithTimeout(ctx, git.Timeout)
		changedFiles, fileList, err := working.ChangedFiles(ctx, oldTagRev)
		if err == nil && len(changedFiles) > 0 {

			for _, file := range fileList {
				commit, err := working.FileLastCommit(ctx, file)
				if err != nil {
					glog.Errorf("get file commit error : v%", err)
					continue
				}
				filesCommits = append(filesCommits, FileCommit{File: file, Commit: commit})
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

		// 给所有发生变化的资源添加label
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

	// 开始同步
	err = Sync(namespace, manifests, allResources, changedResources, g.cluster)

	if err != nil {
		glog.Errorf("sync: %v", err)
		switch syncerr := err.(type) {
		case kubernetes.SyncError:
			for _, e := range syncerr {
				syncErrors = append(syncErrors, ResourceError{
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

	// 更新出错的文件的提交commit
	for i, _ := range syncErrors {
		if fileCommitMap[syncErrors[i].Path] != "" {
			syncErrors[i].Commit = fileCommitMap[syncErrors[i].Path]
		} else {
			ctx, cancel := context.WithTimeout(ctx, git.Timeout)
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
	resourceCommits := make([]ResourceCommit, 0)

	resourceIdList := resourceIDs.ToSlice()

	for _, resourceId := range resourceIdList {
		resourceCommit := ResourceCommit{
			ResourceId: resourceId.String(),
			File:       changedResources[resourceId.String()].Source(),
			Commit:     fileCommitMap[changedResources[resourceId.String()].Source()],
		}
		resourceCommits = append(resourceCommits, resourceCommit)
	}

	if err := g.LogEvent(Event{
		ResourceIDs: resourceIDs.ToSlice(),
		Type:        EventSync,
		StartedAt:   started,
		EndedAt:     started,
		Metadata: &SyncEventMetadata{
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
		ctx, cancel := context.WithTimeout(ctx, git.Timeout)
		// 将远程配置库agent-sync的tag更新到和devops-sync的tag一致
		err := working.MoveSyncTagAndPush(ctx, newTagRev, "Sync pointer")
		cancel()
		if err != nil {
			return err
		}
	}
	// 再次拉取远程仓库，刷新本地仓库
	{
		glog.Infof("%s tag: %s, old: %s, new: %s", namespace, g.gitConfig.SyncTag, oldTagRev, newTagRev)
		ctx, cancel := context.WithTimeout(ctx, git.Timeout)
		err := g.gitRepos[namespace].Refresh(ctx)
		cancel()
		return err
	}
}

// Sync synchronises the cluster to the files in a directory
func Sync(namespace string, m *kubernetes.Manifests, repoResources map[string]resource.Resource, changedResources map[string]resource.Resource, clus *kubernetes.Cluster) error {
	// Get a map of resources defined in the cluster

	// 取出集群中存在的且被agent进行管理的资源
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
	sync := kubernetes.SyncDef{}

	// 比较得出配置库被删掉但还没同步的资源
	for id, res := range clusterResources {
		prepareSyncDelete(repoResources, id, res, &sync)
	}

	// 比较得出配置库中新增或更新的资源
	for _, res := range changedResources {
		prepareSyncApply(res, &sync)
	}

	// 将更新同步到集群
	return clus.Sync(namespace, sync)
}

// todo: remove
func SyncAll(namespace string, m *kubernetes.Manifests, repoResources map[string]resource.Resource, clus kubernetes.Cluster) error {
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
	sync := kubernetes.SyncDef{}

	for id, res := range clusterResources {
		prepareSyncDelete(repoResources, id, res, &sync)
	}

	for _, res := range repoResources {
		prepareSyncApply(res, &sync)
	}
	return clus.Sync(namespace, sync)
}

func prepareSyncApply(res resource.Resource, sync *kubernetes.SyncDef) {
	sync.Actions = append(sync.Actions, kubernetes.SyncAction{
		Apply: res,
	})
}

func prepareSyncDelete(repoResources map[string]resource.Resource, id string, res resource.Resource, sync *kubernetes.SyncDef) {
	//if len(repoResources) == 0 {
	//	return
	//}
	if _, ok := repoResources[id]; !ok {
		sync.Actions = append(sync.Actions, kubernetes.SyncAction{
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
