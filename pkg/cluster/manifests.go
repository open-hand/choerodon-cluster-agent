// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package cluster

import (
	"fmt"

	"github.com/choerodon/choerodon-cluster-agent/pkg/resource"
)

type ManifestError struct {
	error
}

func ErrResourceNotFound(name string) error {
	return ManifestError{fmt.Errorf("manifest for resource %s not found under manifests path", name)}
}

// Manifests represents how a set of files are used as definitions of
// resources, e.g., in Kubernetes, YAML files describing Kubernetes
// resources.
type Manifests interface {
	// Load all the resource manifests under the path given. `baseDir`
	// is used to relativise the paths, which are supplied as absolute
	// paths to directories or files; at least one path must be
	// supplied.
	LoadManifests(baseDir, first string, rest ...string) (map[string]resource.Resource, []string,  error)
	// Parse the manifests given in an exported blob
	ParseManifests([]byte) (map[string]resource.Resource, error)
}
