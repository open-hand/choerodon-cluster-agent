// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package kubernetes

import (
	kube_resource "github.com/choerodon/choerodon-agent/pkg/cluster/kubernetes/resource"
	"github.com/choerodon/choerodon-agent/pkg/resource"
)

type Manifests struct {
	Namespace string
}

func (c *Manifests) LoadManifests(base, first string, rest ...string) (map[string]resource.Resource, []string,  error) {
	return kube_resource.Load(c.Namespace, base, first, rest...)
}

func (c *Manifests) ParseManifests(allDefs []byte) (map[string]resource.Resource, error) {
	return kube_resource.ParseMultidoc(c.Namespace, allDefs, "exported")
}
