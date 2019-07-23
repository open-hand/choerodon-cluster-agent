// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package kubernetes

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/kubernetes/resource"
	resourceutil "github.com/choerodon/choerodon-cluster-agent/pkg/util/resource"
)

type Manifests struct {
}

func (c *Manifests) LoadManifests(namespace string, base, first string, rest ...string) (map[string]resourceutil.Resource, []string, error) {
	return resource.Load(namespace, base, first, rest...)
}

func (c *Manifests) ParseManifests(namespace string, allDefs []byte) (map[string]resourceutil.Resource, error) {
	return resource.ParseMultidoc(namespace, allDefs, "exported")
}
