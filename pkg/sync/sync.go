// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package sync

import (
	"github.com/pkg/errors"

	"github.com/choerodon/choerodon-cluster-agent/pkg/cluster"
	"github.com/choerodon/choerodon-cluster-agent/pkg/resource"
)

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

func prepareSyncApply(res resource.Resource, sync *cluster.SyncDef) {
	sync.Actions = append(sync.Actions, cluster.SyncAction{
		Apply: res,
	})
}
