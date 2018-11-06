// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package cluster

// The things we can get from the running cluster. These used to form
// the remote.Platform interface; but now we do more in the daemon so they
// are distinct interfaces.
type Cluster interface {
	Export(namespace string) ([]byte, error)
	Sync(namespace string, syncDef SyncDef) error
}
