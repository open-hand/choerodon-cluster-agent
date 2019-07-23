// Copyright 2016 Weaveworks Ltd.
// Use of this source code is governed by a Apache License Version 2.0 license
// that can be found at https://github.com/weaveworks/flux/blob/master/LICENSE

package git

import (
	"github.com/choerodon/choerodon-cluster-agent/pkg/util/errors"
)

func PushError(url string, actual error) error {
	return &errors.Error{
		Type: errors.User,
		Err:  actual,
		Help: `Problem committing and pushing to git repository.

There was a problem with committing changes and pushing to the git
repository.

If this has worked before, it most likely means a fast-forward push
was not possible. It is safe to try again.

If it has not worked before, this probably means that the repository
exists but the SSH (deploy) key provided doesn't have write
permission.

`,
	}
}
